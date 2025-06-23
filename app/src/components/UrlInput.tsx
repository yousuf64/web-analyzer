import { useState } from 'react';

interface UrlInputProps {
  onSubmit: (url: string) => Promise<void>;
  isLoading: boolean;
}

export function UrlInput({ onSubmit, isLoading }: UrlInputProps) {
  const [url, setUrl] = useState('');
  const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!url.trim()) {
      setError('Please enter a URL');
      return;
    }

    try {
      new URL(url);
    } catch {
      setError('Please enter a valid URL');
      return;
    }

    try {
      await onSubmit(url);
      setUrl('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ran into an error while starting the analysis');
    }
  };

  return (
    <div className="bg-white shadow-md p-6 border border-gray-200">
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label htmlFor="url" className="block font-medium mb-2">
            Link to analyze
          </label>
          <input
            type="url"
            id="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://google.com"
            className="w-full px-3 py-2 border border-gray-300 shadow-sm focus:outline-none focus:ring-2"
            disabled={isLoading}
          />
          {error && (
            <p className="mt-1 text-sm text-red-600">{error}</p>
          )}
        </div>

        <button
          type="submit"
          disabled={isLoading}
          className="w-full bg-blue-500 text-white py-2 px-4 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {isLoading ? 'Analyzing...' : 'Analyze'}
        </button>
      </form>
    </div>
  );
} 