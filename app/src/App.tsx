import { useState, useEffect } from 'react';
import { ApiService } from './services/api';
import { UrlInput } from './components/UrlInput';
import { JobCard } from './components/JobCard';
import type { Job, JobStatus } from './types';
import { webSocketService } from './services/ws';

function App() {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [newJobIds, setNewJobIds] = useState<Set<string>>(new Set());

  useEffect(() => {
    webSocketService.connect();

    const unsubscribeJob = webSocketService.subscribeToJobUpdates((jobId, status, result) => {
      setJobs(prevJobs => {
        const jobIndex = prevJobs.findIndex(job => job.id === jobId);

        if (jobIndex === -1) {
          // If we get an update for a job we don't know about, fetch all jobs
          ApiService.getJobs()
            .then(allJobs => setJobs(allJobs))
            .catch(err => console.error('Failed to fetch jobs after update:', err));
          return prevJobs;
        } else {
          // Update existing job
          const newJobs = [...prevJobs];
          newJobs[jobIndex] = {
            ...newJobs[jobIndex],
            status: status as JobStatus,
            result: result || newJobs[jobIndex].result
          };
          return newJobs;
        }
      });
    });

    // Initial load of jobs
    setLoading(true);
    ApiService.getJobs()
      .then(jobs => {
        setJobs(jobs);
        setLoading(false);
      })
      .catch(error => {
        setError(error.message);
        setLoading(false);
      });

    return () => {
      unsubscribeJob();
    };
  }, []);

  const handleJobCreated = (job: Job) => {
    setJobs(prevJobs => [job, ...prevJobs]);
    setNewJobIds(prev => new Set([...prev, job.id]));
  };

  if (loading) {
    return <div>Loading jobs...</div>;
  }

  if (error) {
    return <div>Error: {error}</div>;
  }

  return (
    <div className="container mx-auto px-20 py-14">
      <div className="bg-gray-50 border border-gray-200 shadow-md rounded-md p-8 mb-8">
        <div className="flex flex-col items-center justify-center mb-8">
          <h1 className="text-4xl font-bold mb-2">go peek</h1>
          <p className="text-sm text-gray-500 mb-4">
            analyze and extract details from any website.
          </p>
        </div>

        <div>
          <UrlInput onJobCreated={handleJobCreated} />
        </div>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 rounded-md p-4 mb-6">
          <p className="text-red-800">{error}</p>
        </div>
      )}
      <div className="space-y-4">
        {jobs.map(job => (
          <JobCard
            key={job.id}
            job={job}
            isNew={newJobIds.has(job.id)}
          />
        ))}
        {jobs.length === 0 && (
          <div>No jobs yet. Add a URL to analyze.</div>
        )}
      </div>
    </div>
  );
}

export default App;
