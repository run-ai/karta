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
