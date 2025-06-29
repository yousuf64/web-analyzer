import { useState, useEffect } from 'react';
import { ApiService } from './services/api';
import { UrlInput } from './components/UrlInput';
import { JobCard } from './components/JobCard/JobCard';
import type { Job, JobStatus } from './types';
import { webSocketService } from './services/ws';
import Logo from './components/Logo';

const JobCardSkeleton = () => (
  <div className="bg-white shadow-md border border-gray-200 rounded-lg p-4">
    <div className="flex justify-between items-center animate-pulse">
      <div className="flex-grow pr-4">
        <div className="h-4 bg-gray-300 rounded w-3/4 mb-3"></div>
        <div className="h-1.5 bg-gray-300 rounded w-full"></div>
      </div>
      <div className="flex items-center space-x-4">
        <div className="h-5 bg-gray-300 rounded-full w-20"></div>
        <div className="w-6 h-6 bg-gray-300 rounded-full"></div>
      </div>
    </div>
  </div>
);

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

  return (
    <div className="min-h-screen bg-gray-50 text-gray-800">
      <header className="sticky top-0 z-10 p-4 border-b border-gray-300 bg-gray-50/80 backdrop-blur-md">
        <div className="container mx-auto flex justify-between items-center">
          <Logo />
        </div>
      </header>

      <main className="container mx-auto p-4 md:p-8">
        <div className="max-w-3xl mx-auto mb-12">
          <h2 className="text-3xl font-bold text-center mb-2">
            Analyze Any Website
          </h2>
          <p className="text-gray-600 text-center mb-8">
            Enter a URL below to get a detailed analysis of its structure,
            content, and more.
          </p>

          <UrlInput onJobCreated={handleJobCreated} />
        </div>

        <div className="max-w-3xl mx-auto">
            {error && (
              <div className="bg-red-50 border border-red-200 rounded-md p-4 mb-6">
                <p className="text-red-800">{error}</p>
              </div>
            )}
            <div className="space-y-4">
              {loading ? (
                <>
                  <JobCardSkeleton />
                  <JobCardSkeleton />
                  <JobCardSkeleton />
                </>
              ) : (
                <>
                  {jobs.map(job => (
                    <JobCard
                      key={job.id}
                      job={job}
                      isNew={newJobIds.has(job.id)}
                    />
                  ))}
                  {jobs.length === 0 && (
                    <div className="text-center py-10">
                      <p className="text-gray-500">No jobs yet. Add a URL to begin analyzing.</p>
                    </div>
                  )}
                </>
              )}
            </div>
          </div>
      </main>
    </div>
  );
}

export default App;
