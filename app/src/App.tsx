import { useState, useEffect } from 'react';
import type { Job } from './types';
import { ApiService } from './api';
import { UrlInput } from './components/UrlInput';
import { JobCard } from './components/JobCard';

function App() {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');

  const fetchJobs = async () => {
    try {
      const jobList = await ApiService.getJobs();
      setJobs(jobList.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()));
      setError('');
    } catch (err) {
      console.error(err);
      setError('Ran into an error while fetching jobs');
    }
  };

  const handleSubmitUrl = async (url: string) => {
    setIsLoading(true);
    try {
      await ApiService.createAnalyzeJob(url);
      await fetchJobs();
      setError('');
    } catch (err) {
      throw err;
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchJobs();

    const interval = setInterval(fetchJobs, 5000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="bg-gray-50 p-8">
      <h1 className="mb-8 text-3xl font-bold">
        Web Analyzer
      </h1>

      <div className="mb-8">
        <UrlInput onSubmit={handleSubmitUrl} isLoading={isLoading} />
      </div>

      <h2 className="text-2xl font-semibold mb-4">
        Analysis Jobs
      </h2>

      {error && (
        <div className="bg-red-50 border border-red-200 rounded-md p-4 mb-6">
          <p className="text-red-800">{error}</p>
        </div>
      )}

      {jobs.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-lg">
            No analysis jobs yet!
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {jobs.map((job) => (
            <JobCard key={job.id} job={job} />
          ))}
        </div>
      )}
    </div>
  );
}

export default App;
