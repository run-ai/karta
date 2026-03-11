import { Editor } from '@monaco-editor/react';

interface EditorPanelProps {
  title: string;
  value: string;
  onChange: (value: string) => void;
  language?: string;
  readOnly?: boolean;
}

export function EditorPanel({ 
  title, 
  value, 
  onChange, 
  language = 'yaml',
  readOnly = false 
}: EditorPanelProps) {
  return (
    <div className="flex flex-col h-full">
      <div className="bg-gray-800 text-white px-4 py-2 text-sm font-semibold border-b border-gray-700">
        {title}
      </div>
      <div className="flex-1 overflow-hidden">
        <Editor
          height="100%"
          language={language}
          value={value}
          onChange={(newValue) => onChange(newValue || '')}
          theme="vs-dark"
          options={{
            minimap: { enabled: false },
            fontSize: 13,
            lineNumbers: 'on',
            readOnly,
            scrollBeyondLastLine: false,
            wordWrap: 'on',
            automaticLayout: true,
          }}
        />
      </div>
    </div>
  );
}




