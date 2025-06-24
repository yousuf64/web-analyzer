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
  };

  if (loading) {
    return <div>Loading jobs...</div>;
  }

  if (error) {
    return <div>Error: {error}</div>;
  }

  return (
    <div className="container mx-auto p-4">
      <h1 className="text-2xl font-bold mb-4">Web Analyzer</h1>
      
      <div className="mb-8">
        <UrlInput onJobCreated={handleJobCreated} />
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
