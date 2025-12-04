import { useState } from 'react';
import { ComponentResult as ComponentResultType } from '../types';

interface ComponentResultProps {
  component: ComponentResultType;
}

export function ComponentResult({ component }: ComponentResultProps) {
  const [isExpanded, setIsExpanded] = useState(false);

  const hasData = 
    component.podTemplateSpec || 
    component.podSpec || 
    component.fragmentedSpec || 
    component.scale || 
    (component.instanceIds && component.instanceIds.length > 0);

  return (
    <div className="bg-gray-800 rounded-lg overflow-hidden">
      <div 
        className="px-4 py-3 flex items-center justify-between cursor-pointer hover:bg-gray-750"
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <div className="flex items-center gap-3">
          <span className="text-white font-semibold">{component.name}</span>
          {component.kind && (
            <span className="text-gray-400 text-sm">
              {component.kind.kind} ({component.kind.group}/{component.kind.version})
            </span>
          )}
        </div>
        <span className="text-gray-400 text-sm">
          {isExpanded ? '▼' : '▶'}
        </span>
      </div>

      {isExpanded && (
        <div className="px-4 py-3 bg-gray-900 space-y-3">
          {component.error && (
            <div className="bg-red-900 border border-red-700 text-red-100 px-3 py-2 rounded text-sm">
              Error: {component.error}
            </div>
          )}

          {component.instanceIds && component.instanceIds.length > 0 && (
            <div>
              <h4 className="text-gray-300 font-medium text-sm mb-1">Instance IDs</h4>
              <div className="bg-gray-800 rounded p-2">
                <code className="text-green-400 text-xs">
                  {JSON.stringify(component.instanceIds, null, 2)}
                </code>
              </div>
            </div>
          )}

          {component.podTemplateSpec && (
            <DataSection title="Pod Template Spec" data={component.podTemplateSpec} />
          )}

          {component.podSpec && (
            <DataSection title="Pod Spec" data={component.podSpec} />
          )}

          {component.podMetadata && (
            <DataSection title="Pod Metadata" data={component.podMetadata} />
          )}

          {component.fragmentedSpec && (
            <DataSection title="Fragmented Pod Spec" data={component.fragmentedSpec} />
          )}

          {component.scale && (
            <DataSection title="Scale" data={component.scale} />
          )}

          {!hasData && !component.error && (
            <div className="text-gray-400 text-sm italic">
              No data extracted for this component
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function DataSection({ title, data }: { title: string; data: any }) {
  const [isExpanded, setIsExpanded] = useState(false);

  return (
    <div>
      <div 
        className="flex items-center justify-between cursor-pointer mb-1"
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <h4 className="text-gray-300 font-medium text-sm">{title}</h4>
        <span className="text-gray-500 text-xs">
          {isExpanded ? 'Hide' : 'Show'}
        </span>
      </div>
      {isExpanded && (
        <div className="bg-gray-800 rounded p-3 overflow-auto max-h-96">
          <pre className="text-green-400 text-xs">
            {JSON.stringify(data, null, 2)}
          </pre>
        </div>
      )}
    </div>
  );
}




