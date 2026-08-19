// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package controller

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"

	"github.com/run-ai/karta/exporter/pkg/collector"
	"github.com/run-ai/karta/exporter/pkg/owner"
	"github.com/run-ai/karta/exporter/pkg/registry"
	"github.com/run-ai/karta/exporter/pkg/store"
)

var kartaGVR = schema.GroupVersionResource{Group: "run.ai", Version: "v1alpha1", Resource: "kartas"}

var podGroupKind = schema.GroupKind{Group: "", Kind: "Pod"}

// Options configures the controller.
type Options struct {
	// FullPodCache disables the pod cache trim, for custom Kartas whose pod
	// selectors read fields outside metadata, spec.nodeName, and status.phase.
	FullPodCache bool
	Resync       time.Duration
}

// Controller watches Karta CRs, Karta-described workloads, their child
// objects, and pods, and keeps the store current. All metric rendering
// happens in the collector; the controller only reacts to watch events.
type Controller struct {
	dynamicClient  dynamic.Interface
	metadataClient metadata.Interface
	kubeClient     kubernetes.Interface
	mapper         *restmapper.DeferredDiscoveryRESTMapper
	options        Options
	logger         *slog.Logger

	registry *registry.Registry
	index    *owner.Index
	store    *store.Store

	podInformer   cache.SharedIndexInformer
	kartaInformer cache.SharedIndexInformer

	mu       sync.Mutex
	watchers map[schema.GroupVersionKind]*watcher

	attributionErrors *prometheus.CounterVec
	lastEvent         prometheus.Gauge

	startedMu sync.RWMutex
	started   bool
}

type watcher struct {
	gvk      schema.GroupVersionKind
	informer cache.SharedIndexInformer
	stop     chan struct{}
	isRoot   bool
}

func New(
	dynamicClient dynamic.Interface,
	metadataClient metadata.Interface,
	kubeClient kubernetes.Interface,
	mapper *restmapper.DeferredDiscoveryRESTMapper,
	reg *registry.Registry,
	index *owner.Index,
	s *store.Store,
	registerer prometheus.Registerer,
	logger *slog.Logger,
	options Options,
) *Controller {
	controller := &Controller{
		dynamicClient:  dynamicClient,
		metadataClient: metadataClient,
		kubeClient:     kubeClient,
		mapper:         mapper,
		options:        options,
		logger:         logger,
		registry:       reg,
		index:          index,
		store:          s,
		watchers:       make(map[schema.GroupVersionKind]*watcher),
		attributionErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: collector.MetricExporterAttributionErrors,
			Help: "Attribution and status evaluation errors, by reason.",
		}, []string{collector.LabelReason}),
		lastEvent: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: collector.MetricExporterLastEventSeconds,
			Help: "Unix timestamp of the last processed watch event. A freshness witness.",
		}),
	}
	registerer.MustRegister(controller.attributionErrors, controller.lastEvent)
	return controller
}

// Ready reports whether all informers, including per-workload-kind watchers,
// have synced at least once.
func (c *Controller) Ready() bool {
	c.startedMu.RLock()
	started := c.started
	c.startedMu.RUnlock()
	if !started {
		return false
	}
	if !c.kartaInformer.HasSynced() || !c.podInformer.HasSynced() {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, w := range c.watchers {
		if !w.informer.HasSynced() {
			return false
		}
	}
	return true
}

// Run starts the Karta and pod informers and blocks until the context ends.
func (c *Controller) Run(ctx context.Context) error {
	factory := informers.NewSharedInformerFactoryWithOptions(
		c.kubeClient, c.options.Resync,
		informers.WithTransform(c.podTransform()),
	)
	c.podInformer = factory.Core().V1().Pods().Informer()
	if _, err := c.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { c.onPodEvent(obj) },
		UpdateFunc: func(oldObj, newObj any) { c.onPodUpdate(oldObj, newObj) },
		DeleteFunc: func(obj any) { c.onPodDelete(obj) },
	}); err != nil {
		return fmt.Errorf("failed to add pod handler: %w", err)
	}

	c.kartaInformer = c.newDynamicInformer(kartaGVR, false)
	if _, err := c.kartaInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { c.onKartaEvent(obj) },
		UpdateFunc: func(_, newObj any) { c.onKartaEvent(newObj) },
		DeleteFunc: func(obj any) { c.onKartaDelete(obj) },
	}); err != nil {
		return fmt.Errorf("failed to add karta handler: %w", err)
	}

	factory.Start(ctx.Done())
	go c.kartaInformer.RunWithContext(ctx)

	if !cache.WaitForCacheSync(ctx.Done(), c.podInformer.HasSynced, c.kartaInformer.HasSynced) {
		return fmt.Errorf("informer caches failed to sync")
	}

	c.startedMu.Lock()
	c.started = true
	c.startedMu.Unlock()

	<-ctx.Done()
	c.stopAllWatchers()
	return nil
}

// podTransform trims cached pods to what pod selectors read: metadata,
// spec.nodeName, and status.phase. Custom Kartas reading other pod fields
// need the FullPodCache escape hatch.
func (c *Controller) podTransform() cache.TransformFunc {
	if c.options.FullPodCache {
		return func(obj any) (any, error) { return obj, nil }
	}
	return func(obj any) (any, error) {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return obj, nil
		}
		return &corev1.Pod{
			TypeMeta: pod.TypeMeta,
			ObjectMeta: metav1.ObjectMeta{
				Name:              pod.Name,
				Namespace:         pod.Namespace,
				UID:               pod.UID,
				ResourceVersion:   pod.ResourceVersion,
				Labels:            pod.Labels,
				Annotations:       pod.Annotations,
				OwnerReferences:   pod.OwnerReferences,
				CreationTimestamp: pod.CreationTimestamp,
			},
			Spec:   corev1.PodSpec{NodeName: pod.Spec.NodeName},
			Status: corev1.PodStatus{Phase: pod.Status.Phase},
		}, nil
	}
}

func (c *Controller) newDynamicInformer(gvr schema.GroupVersionResource, isMetadata bool) cache.SharedIndexInformer {
	if isMetadata {
		return cache.NewSharedIndexInformer(&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return c.metadataClient.Resource(gvr).Namespace(metav1.NamespaceAll).List(context.Background(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return c.metadataClient.Resource(gvr).Namespace(metav1.NamespaceAll).Watch(context.Background(), options)
			},
		}, &metav1.PartialObjectMetadata{}, c.options.Resync, cache.Indexers{})
	}
	return cache.NewSharedIndexInformer(&cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return c.dynamicClient.Resource(gvr).Namespace(metav1.NamespaceAll).List(context.Background(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return c.dynamicClient.Resource(gvr).Namespace(metav1.NamespaceAll).Watch(context.Background(), options)
		},
	}, &unstructured.Unstructured{}, c.options.Resync, cache.Indexers{})
}

// reconcileWatchers aligns the running per-kind informers with the registry:
// one full informer per chosen root kind, one metadata informer per child
// kind. Pods are excluded; the dedicated pod informer covers them.
func (c *Controller) reconcileWatchers() {
	desiredRoots := make(map[schema.GroupVersionKind]struct{})
	desiredChildren := make(map[schema.GroupVersionKind]struct{})
	for _, entry := range c.registry.Entries() {
		desiredRoots[entry.RootGVK] = struct{}{}
		for _, child := range entry.ChildKinds {
			if child.GroupKind() == podGroupKind {
				continue
			}
			desiredChildren[child] = struct{}{}
		}
	}
	for gvk := range desiredRoots {
		delete(desiredChildren, gvk)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for gvk, w := range c.watchers {
		_, wantRoot := desiredRoots[gvk]
		_, wantChild := desiredChildren[gvk]
		if (w.isRoot && wantRoot) || (!w.isRoot && wantChild) {
			continue
		}
		close(w.stop)
		delete(c.watchers, gvk)
		c.logger.Info("stopped watcher", "gvk", gvk.String())
	}

	for gvk := range desiredRoots {
		if _, ok := c.watchers[gvk]; !ok {
			c.startWatcherLocked(gvk, true)
		}
	}
	for gvk := range desiredChildren {
		if _, ok := c.watchers[gvk]; !ok {
			c.startWatcherLocked(gvk, false)
		}
	}
}

func (c *Controller) startWatcherLocked(gvk schema.GroupVersionKind, isRoot bool) {
	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		c.mapper.Reset()
		mapping, err = c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	}
	if err != nil {
		c.logger.Error("cannot map kind to a resource, skipping watcher", "gvk", gvk.String(), "error", err)
		return
	}

	informer := c.newDynamicInformer(mapping.Resource, !isRoot)
	w := &watcher{gvk: gvk, informer: informer, stop: make(chan struct{}), isRoot: isRoot}

	var handlerErr error
	if isRoot {
		_, handlerErr = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj any) { c.onWorkloadEvent(obj) },
			UpdateFunc: func(_, newObj any) { c.onWorkloadEvent(newObj) },
			DeleteFunc: func(obj any) { c.onWorkloadDelete(obj) },
		})
	} else {
		_, handlerErr = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj any) { c.onChildEvent(obj) },
			UpdateFunc: func(_, newObj any) { c.onChildEvent(newObj) },
			DeleteFunc: func(obj any) { c.onChildDelete(obj) },
		})
	}
	if handlerErr != nil {
		c.logger.Error("failed to add watcher handler", "gvk", gvk.String(), "error", handlerErr)
		return
	}

	c.watchers[gvk] = w
	go informer.Run(w.stop)
	c.logger.Info("started watcher", "gvk", gvk.String(), "resource", mapping.Resource.String(), "root", isRoot)
}

func (c *Controller) stopAllWatchers() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for gvk, w := range c.watchers {
		close(w.stop)
		delete(c.watchers, gvk)
	}
}

func (c *Controller) rootWatcherStore(groupKind schema.GroupKind) cache.Store {
	c.mu.Lock()
	defer c.mu.Unlock()
	for gvk, w := range c.watchers {
		if w.isRoot && gvk.GroupKind() == groupKind {
			return w.informer.GetStore()
		}
	}
	return nil
}

func (c *Controller) markEvent() {
	c.lastEvent.SetToCurrentTime()
}
