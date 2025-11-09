# Resource Interface
## What is a Resource Interface?
In Kubernetes, a workload is not a standalone execution unit (pod). Instead, it comprises various components, including an ingress entry point, a collection of pods, and storage.

The purpose of the Resource Interface (RI) is to allow to perform actions and extract information from a new workload type. RI enables any controller to manage, monitor, and interact with new and custom Kubernetes workload types. By registering a workload type via an RI, the code can perform resource allocation, scheduling, monitoring, and data extraction, ensuring efficient operation and seamless integration.

A Resource Interface (RI) is a structured description of a Kubernetes workload type.

It tells the user how to:
- Identify the root component (the described CRD itself)
- Model child components (replicas, workers, statefulsets)
- Locate pod specs inside the resource definition
- Interpret status (running, completed, failed)
- Apply op§imization instructions (gang scheduling)
- Think of it as the blueprint of the workload.
