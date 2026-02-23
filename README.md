# Resource Interface
## What is a Resource Interface?
In Kubernetes, a workload is not a standalone execution unit (pod). Instead, it comprises various components, including an ingress entry point, a collection of pods, and storage.

The purpose of the Resource Interface (RI) is to allow to perform actions and extract information from a new workload type. RI enables any controller to manage, monitor, and interact with new and custom Kubernetes workload types. By registering a workload type via an RI, the code can perform resource allocation, scheduling, monitoring, and data extraction, ensuring efficient operation and seamless integration.

A Resource Interface (RI) is a structured mapping of a Kubernetes workload type.

It tells the user how to:
- Identify the root component (the described CRD itself)
- Model child components (replicas, workers, statefulsets)
- Locate pod specs inside the resource definition
- Interpret status (running, completed, failed)
- Apply optimization instructions (gang scheduling)
- Think of it as the blueprint of the workload.

## Usage
Use the Component API to programmatically extract workload information from specific Kubernetes resource using its ResourceInterface.

### Example: JobSet with RI

**JobSet Object:**
```yaml
apiVersion: jobset.x-k8s.io/v1alpha2
kind: JobSet
metadata:
  name: my-training-job
spec:
  replicatedJobs:
  - name: master
    replicas: 1
    template:
      spec:
        template:
          spec:
            containers:
            - name: trainer
              image: my-training:latest
  - name: worker
    replicas: 3
    template:
      spec:
        template:
          spec:
            containers:
            - name: trainer
              image: my-training:latest
```

**ResourceInterface Definition:**
```yaml
apiVersion: optimization.nvidia.com/v1alpha1
kind: ResourceInterface
spec:
  structureDefinition:
    rootComponent:
      name: "jobset"
      kind:
        group: "jobset.x-k8s.io"
        version: "v1alpha2"
        kind: "JobSet"
      statusDefinition:
        conditionsDefinition:
          path: ".status.conditions"
          typeFieldName: "type"
          statusFieldName: "status"
        statusMappings:
          running:
          - byConditions:
            - type: "StartupPolicyCompleted"
              status: "True"
          completed:
          - byConditions:
            - type: "Completed"
              status: "True"
          failed:
          - byConditions:
            - type: "Failed"
              status: "True"
    childComponents:
    - name: "replicatedjob"
      specDefinition:
        podTemplateSpecPath: ".spec.replicatedJobs[].template"
      scaleDefinition:
        replicasPath: ".spec.replicatedJobs[].replicas"
      instanceIdPath: ".spec.replicatedJobs[].name"  # Instances: "master", "worker"
```

### Basic Extraction
```go
import "github.com/run-ai/kai-bolt/pkg/resource"

// Create a factory from your ResourceInterface and JobSet object
factory := resource.NewComponentFactoryFromObject(resourceInterface, jobSetObject)

// Get the child component (replicatedjob) which has the instances
component, _ := factory.GetComponent("replicatedjob")
summaries, _ := component.GetExtractedInstances(ctx)

// Access pod template specs, metadata, and scale info for each instance
for instanceID, summary := range summaries {
    // instanceID will be "master" or "worker" in our example
    if summary.PodTemplateSpec != nil {
        // Work with pod template specs
    }
}

// Get status from the root component (jobset)
rootComponent, _ := factory.GetRootComponent()
status, _ := rootComponent.GetStatus(ctx)
// status.MatchedStatuses: []ResourceStatus - statuses matched based on conditions (e.g., ["running"])
// status.Phase: raw phase string from the workload
// status.Conditions: []Condition with Type, Status, Message fields
```

### Update Capabilities
The Resource Interface also supports updating the workload resource. You can modify pod specifications, metadata, or specific fields using the Component API.

The same paths defined in `SpecDefinition` are used for both extraction and updates.

```go
// ... assuming factory and component are already created ...

// 1. Prepare the updates
// Map instance IDs to the new values you want to set
updates := map[string]resource.FragmentedPodSpec{
    "master": {
        SchedulerName: "my-custom-scheduler",
        Labels: map[string]string{
            "my-label": "true",
        },
    },
    "worker": {
        SchedulerName: "my-custom-scheduler",
    },
}

// 2. Apply the updates
// This modifies the underlying unstructured object in the factory
err := component.UpdateFragmentedPodSpec(ctx, updates)
if err != nil {
    // Handle error
}

// 3. Get the updated object to apply it back to the cluster
updatedObject, _ := factory.GetObject()
// ... use dynamic client to Update/Patch the object in Kubernetes ...
```

## License and Copyright

This project is licensed under the Apache License, Version 2.0. See the [LICENSE](LICENSE) file for the full license text.

Copyright (c) 2026 NVIDIA Corporation

## Third-Party Software

This project includes third-party software components. See the [NOTICE](NOTICE) file for attributions and the [THIRD_PARTY_LICENSES](THIRD_PARTY_LICENSES) file for detailed license information.

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on how to contribute to this project. All contributions must comply with the Developer Certificate of Origin (DCO).

## Documentation

- [LICENSE](LICENSE) - Apache 2.0 license
- [NOTICE](NOTICE) - Copyright and third-party attributions
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines and DCO
- [THIRD_PARTY_LICENSES](THIRD_PARTY_LICENSES) - Third-party dependency licenses
