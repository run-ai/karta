import { ExtractResponse, ValidateResponse } from '../types';
import { ComponentResult } from './ComponentResult';
import { ErrorDisplay } from './ErrorDisplay';

interface ResultsPanelProps {
  validationResult?: ValidateResponse | null;
  extractionResult?: ExtractResponse | null;
  isCollapsed: boolean;
  onToggle: () => void;
}

export function ResultsPanel({ 
  validationResult, 
  extractionResult, 
  isCollapsed,
  onToggle 
}: ResultsPanelProps) {
  const hasResults = validationResult !== null || extractionResult !== null;

  return (
    <div className={`bg-gray-900 border-t border-gray-700 flex flex-col transition-all duration-300 ${
      isCollapsed ? 'h-10' : 'h-96'
    }`}>
      <div 
        className="bg-gray-800 px-4 py-2 flex items-center justify-between cursor-pointer hover:bg-gray-750"
        onClick={onToggle}
      >
        <h2 className="text-white font-semibold text-sm">Results</h2>
        <button className="text-gray-400 hover:text-white">
          {isCollapsed ? '▲ Expand' : '▼ Collapse'}
        </button>
      </div>

      {!isCollapsed && (
        <div className="flex-1 overflow-auto p-4">
          {!hasResults && (
            <div className="text-gray-400 text-center py-8">
              No results yet. Click "Validate RI" or "Extract" to see results here.
            </div>
          )}

          {validationResult && (
            <div className="mb-6">
              <h3 className="text-white font-semibold mb-2">Validation Results</h3>
              {validationResult.valid ? (
                <div className="bg-green-900 border border-green-700 text-green-100 px-4 py-3 rounded">
                  ✓ RI validation successful
                </div>
              ) : (
                <ErrorDisplay errors={validationResult.errors || []} title="Validation Errors" />
              )}
            </div>
          )}

          {extractionResult && (
            <div>
              <h3 className="text-white font-semibold mb-2">Extraction Results</h3>
              {!extractionResult.success && extractionResult.errors && (
                <ErrorDisplay errors={extractionResult.errors} title="Extraction Errors" />
              )}
              {extractionResult.components && extractionResult.components.length > 0 && (
                <div className="space-y-4 mt-4">
                  {extractionResult.components.map((component, index) => (
                    <ComponentResult key={index} component={component} />
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}




