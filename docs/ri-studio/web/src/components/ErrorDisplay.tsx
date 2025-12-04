interface ErrorDisplayProps {
  errors: string[];
  title?: string;
}

export function ErrorDisplay({ errors, title = 'Errors' }: ErrorDisplayProps) {
  if (errors.length === 0) {
    return null;
  }

  return (
    <div className="bg-red-900 border border-red-700 rounded overflow-hidden">
      <div className="px-4 py-2 bg-red-800 text-red-100 font-semibold text-sm">
        {title} ({errors.length})
      </div>
      <div className="px-4 py-3">
        <ul className="space-y-2">
          {errors.map((error, index) => (
            <li key={index} className="text-red-100 text-sm">
              <span className="text-red-400">•</span> {error}
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}




