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
      name: "example"
      kind:
        group: "example.com"
        version: "v1"
        kind: "Example"
      statusDefinition:
        statusMappings:
          running:
            - byPhase: "Running"
`;

const DEFAULT_CR = `apiVersion: example.com/v1
kind: Example
metadata:
  name: my-example
spec:
  # Add your custom resource spec here
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




