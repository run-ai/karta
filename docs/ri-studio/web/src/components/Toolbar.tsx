import { useState, useEffect } from 'react';
import { ExampleInfo } from '../types';
import { api } from '../services/api';

interface ToolbarProps {
  onValidate: () => void;
  onExtract: () => void;
  onLoadExample: (name: string) => void;
  isLoading: boolean;
}

export function Toolbar({ onValidate, onExtract, onLoadExample, isLoading }: ToolbarProps) {
  const [examples, setExamples] = useState<ExampleInfo[]>([]);
  const [selectedExample, setSelectedExample] = useState<string>('');

  useEffect(() => {
    api.listExamples().then((response) => {
      setExamples(response.examples);
    }).catch((error) => {
      console.error('Failed to load examples:', error);
    });
  }, []);

  const handleExampleChange = (event: React.ChangeEvent<HTMLSelectElement>) => {
    const name = event.target.value;
    setSelectedExample(name);
    if (name) {
      onLoadExample(name);
    }
  };

  return (
    <div className="bg-gray-900 border-b border-gray-700 px-4 py-3 flex items-center gap-4">
      <div className="flex items-center gap-2">
        <h1 className="text-xl font-bold text-white">RI Studio</h1>
        <span className="text-gray-400 text-sm">Resource Interface Authoring Tool</span>
      </div>
      
      <div className="flex-1" />

      <div className="flex items-center gap-3">
        <select
          value={selectedExample}
          onChange={handleExampleChange}
          className="bg-gray-800 text-white border border-gray-600 rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          disabled={isLoading}
        >
          <option value="">Load Example...</option>
          {examples.map((example) => (
            <option key={example.name} value={example.name}>
              {example.displayName}
            </option>
          ))}
        </select>

        <button
          onClick={onValidate}
          disabled={isLoading}
          className="bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 text-white px-4 py-1.5 rounded text-sm font-medium transition-colors"
        >
          Validate RI
        </button>

        <button
          onClick={onExtract}
          disabled={isLoading}
          className="bg-green-600 hover:bg-green-700 disabled:bg-gray-600 text-white px-4 py-1.5 rounded text-sm font-medium transition-colors"
        >
          Extract
        </button>
      </div>
    </div>
  );
}




