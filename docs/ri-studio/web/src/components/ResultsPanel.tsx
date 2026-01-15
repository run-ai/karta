import { useState, useCallback, useEffect } from 'react';
import { ExtractResponse, ValidateResponse } from '../types';
import { ComponentResult } from './ComponentResult';
import { ErrorDisplay } from './ErrorDisplay';
import { HierarchyVisualization } from './HierarchyVisualization';

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
  const [activeTab, setActiveTab] = useState<'results' | 'hierarchy'>('results');
  const [panelHeight, setPanelHeight] = useState(384); // 96 * 4 = 384px (h-96)
  const [isResizing, setIsResizing] = useState(false);
  const hasResults = validationResult !== null || extractionResult !== null;
  const hasComponents = extractionResult?.components && extractionResult.components.length > 0;

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    setIsResizing(true);
  }, []);

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (!isResizing) return;
    const newHeight = window.innerHeight - e.clientY;
    // Min height: 150px, Max height: 80% of window
    if (newHeight >= 150 && newHeight <= window.innerHeight * 0.8) {
      setPanelHeight(newHeight);
    }
  }, [isResizing]);

  const handleMouseUp = useCallback(() => {
    setIsResizing(false);
  }, []);

  useEffect(() => {
    if (isResizing) {
      document.addEventListener('mousemove', handleMouseMove);
      document.addEventListener('mouseup', handleMouseUp);
      return () => {
        document.removeEventListener('mousemove', handleMouseMove);
        document.removeEventListener('mouseup', handleMouseUp);
      };
    }
  }, [isResizing, handleMouseMove, handleMouseUp]);

  return (
    <div 
      className="bg-gray-900 border-t border-gray-700 flex flex-col transition-all duration-300"
      style={{
        height: isCollapsed ? '40px' : `${panelHeight}px`,
        minHeight: isCollapsed ? '40px' : '150px'
      }}
    >
      {/* Resize Handle */}
      {!isCollapsed && (
        <div
          className="h-1 bg-gray-700 hover:bg-blue-500 cursor-ns-resize transition-colors"
          onMouseDown={handleMouseDown}
        />
      )}
      
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
        <div className="flex flex-col flex-1 min-h-0">
          {/* Tab Navigation */}
          {hasComponents && (
            <div className="bg-gray-800 border-b border-gray-700 px-4 py-2 flex gap-2">
              <button
                onClick={() => setActiveTab('results')}
                className={`px-4 py-2 rounded text-sm font-medium transition-colors ${
                  activeTab === 'results'
                    ? 'bg-gray-700 text-white'
                    : 'text-gray-400 hover:text-white hover:bg-gray-750'
                }`}
              >
                Results
              </button>
              <button
                onClick={() => setActiveTab('hierarchy')}
                className={`px-4 py-2 rounded text-sm font-medium transition-colors ${
                  activeTab === 'hierarchy'
                    ? 'bg-gray-700 text-white'
                    : 'text-gray-400 hover:text-white hover:bg-gray-750'
                }`}
              >
                Hierarchy
              </button>
            </div>
          )}

          {/* Tab Content */}
          <div className="flex-1 overflow-auto min-h-0">
            {activeTab === 'results' && (
              <div className="p-4">
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
                    {hasComponents && extractionResult.components && (
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

            {activeTab === 'hierarchy' && hasComponents && extractionResult.components && (
              <HierarchyVisualization components={extractionResult.components} />
            )}
          </div>
        </div>
      )}
    </div>
  );
}




