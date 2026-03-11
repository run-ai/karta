import { useState } from 'react';
import { EditorPanel } from './components/EditorPanel';
import { Toolbar } from './components/Toolbar';
import { ResultsPanel } from './components/ResultsPanel';
import { ValidateResponse, ExtractResponse } from './types';
import { api } from './services/api';

const DEFAULT_RI = `apiVersion: optimization.nvidia.com/v1alpha1
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
          initializing:
          - byConditions:
            - type: "StartupPolicyInProgress"
              status: "True"
          running:
          - byConditions:
            - type: "StartupPolicyCompleted"
              status: "True"
            - type: "Completed"
              status: "False"
            - type: "Failed"
              status: "False"
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
      kind:
        group: "batch"
        version: "v1"
        kind: "Job"
      ownerRef: "jobset"
      specDefinition:
        podTemplateSpecPath: ".spec.replicatedJobs[].template.spec.template"
      scaleDefinition:
        replicasPath: ".spec.replicatedJobs[].replicas"
      instanceIdPath: ".spec.replicatedJobs[].name"
      podSelector:
        componentInstanceSelector:
          idPath: '.metadata.labels["jobset.sigs.k8s.io/replicatedjob-name"]'

  optimizationInstructions:
    gangScheduling:
      podGroups:
      - name: "job"
        members:
        - componentName: "replicatedjob"
          groupByKeyPaths:
          - '.metadata.labels["jobset.sigs.k8s.io/replicatedjob-name"]'
`;

const DEFAULT_CR = `apiVersion: jobset.x-k8s.io/v1alpha2
kind: JobSet
metadata:
  name: pytorch-distributed-training
spec:
  # 1. Network: Automatically creates a headless service for pod-to-pod communication
  network:
    enableDNSHostnames: true
    subdomain: pytorch-svc # Pods reach each other at: <pod-name>.pytorch-svc

  # 2. Success Policy: The JobSet is "Done" when the 'driver' finishes.
  #    (Workers run indefinitely until the driver kills them or finishes).
  successPolicy:
    operator: Any
    targetReplicatedJobs:
      - driver

  failurePolicy:
    maxRestarts: 3

  replicatedJobs:
    # --- Group A: The Driver (Leader) ---
    - name: driver
      replicas: 1
      template:
        spec:
          parallelism: 1
          completions: 1
          backoffLimit: 0
          template:
            spec:
              containers:
              - name: pytorch
                image: pytorch/pytorch:latest
                command: ["python", "train_script.py", "--role", "master"]
                env:
                - name: MASTER_ADDR
                  value: "pytorch-distributed-training-driver-0-0.pytorch-svc"
              restartPolicy: Never

    # --- Group B: The Workers ---
    - name: workers
      replicas: 4
      template:
        spec:
          parallelism: 1
          completions: 1
          backoffLimit: 0 
          template:
            spec:
              containers:
              - name: pytorch
                image: pytorch/pytorch:latest
                command: ["python", "train_script.py", "--role", "worker"]
                env:
                - name: MASTER_ADDR
                  # Connects to the driver defined above
                  value: "pytorch-distributed-training-driver-0-0.pytorch-svc"
              restartPolicy: Never
    - name: aggregators
      replicas: 5
      template:
        spec:
          parallelism: 1
          completions: 1
          backoffLimit: 0 
          template:
            spec:
              containers:
              - name: pytorch
                image: pytorch/pytorch:latest
                command: ["python", "train_script.py", "--role", "worker"]
                env:
                - name: MASTER_ADDR
                  # Connects to the driver defined above
                  value: "pytorch-distributed-training-driver-0-0.pytorch-svc"
              restartPolicy: Never
`;

function App() {
  const [crYaml, setCrYaml] = useState(DEFAULT_CR);
  const [riYaml, setRiYaml] = useState(DEFAULT_RI);
  const [validationResult, setValidationResult] = useState<ValidateResponse | null>(null);
  const [extractionResult, setExtractionResult] = useState<ExtractResponse | null>(null);
  const [isResultsCollapsed, setIsResultsCollapsed] = useState(true);
  const [isLoading, setIsLoading] = useState(false);

  const handleValidate = async () => {
    setIsLoading(true);
    setValidationResult(null);
    setExtractionResult(null);
    setIsResultsCollapsed(false);

    try {
      const result = await api.validateRI(riYaml);
      setValidationResult(result);
    } catch (error) {
      setValidationResult({
        valid: false,
        errors: [error instanceof Error ? error.message : 'Unknown error occurred'],
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handleExtract = async () => {
    setIsLoading(true);
    setValidationResult(null);
    setExtractionResult(null);
    setIsResultsCollapsed(false);

    try {
      const result = await api.extract(crYaml, riYaml);
      setExtractionResult(result);
    } catch (error) {
      setExtractionResult({
        success: false,
        errors: [error instanceof Error ? error.message : 'Unknown error occurred'],
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handleLoadExample = async (name: string) => {
    setIsLoading(true);
    try {
      const example = await api.getExample(name);
      setRiYaml(example.ri);
      if (example.cr) {
        setCrYaml(example.cr);
      }
      // Clear results when loading a new example
      setValidationResult(null);
      setExtractionResult(null);
    } catch (error) {
      console.error('Failed to load example:', error);
      alert(`Failed to load example: ${error instanceof Error ? error.message : 'Unknown error'}`);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="h-screen flex flex-col bg-gray-900">
      <Toolbar
        onValidate={handleValidate}
        onExtract={handleExtract}
        onLoadExample={handleLoadExample}
        isLoading={isLoading}
      />

      <div className="flex-1 flex overflow-hidden">
        {/* Left Panel - CR Editor */}
        <div className="w-1/2 border-r border-gray-700">
          <EditorPanel
            title="Custom Resource (CR) YAML"
            value={crYaml}
            onChange={setCrYaml}
          />
        </div>

        {/* Right Panel - RI Editor */}
        <div className="w-1/2">
          <EditorPanel
            title="Resource Interface (RI) YAML"
            value={riYaml}
            onChange={setRiYaml}
          />
        </div>
      </div>

      {/* Bottom Panel - Results */}
      <ResultsPanel
        validationResult={validationResult}
        extractionResult={extractionResult}
        isCollapsed={isResultsCollapsed}
        onToggle={() => setIsResultsCollapsed(!isResultsCollapsed)}
      />
    </div>
  );
}

export default App;




