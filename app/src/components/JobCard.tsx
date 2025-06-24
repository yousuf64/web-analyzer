import { useState } from 'react';
import type { Job } from '../types';
import { TaskCard } from './TaskCard';

interface JobCardProps {
  job: Job;
  isNew?: boolean;
}

export function JobCard({ job, isNew = false }: JobCardProps) {
  const [expanded, setExpanded] = useState(isNew);

  return (
    <div className="bg-white shadow-md p-6 border border-gray-200">
      <div className="flex justify-between items-start mb-4">
        <div>
          <h3 className="text-md font-semibold mb-2">
            {job.url}
          </h3>
          <p className="text-sm">
            Job ID: {job.id}
          </p>
        </div>
        <span
          className="text-sm font-medium"
        >
          [{job.status}]
        </span>
      </div>

      <div className="text-sm mb-4">
        <p>Created: {job.created_at.toLocaleString()}</p>
        {job.started_at && <p>Started: {job.started_at.toLocaleString()}</p>}
        {job.completed_at && <p>Completed: {job.completed_at.toLocaleString()}</p>}
      </div>

      {job.result && (
        <div className="pt-4">
          <h4 className="font-semibold mb-2">Results</h4>
          <div className="space-y-2 text-sm">
            <p><span>HTML Version:</span> {job.result.html_version}</p>
            <p><span>Page Title:</span> {job.result.page_title}</p>
            <p><span>Has Login Form:</span> {job.result.has_login_form ? 'Yes' : 'No'}</p>
            <p><span>Links Found:</span> {job.result.links.length}</p>

            {Object.keys(job.result.headings).length > 0 && (
              <div>
                <span>Headings:</span>
                <div className="ml-4 mt-1">
                  {Object.entries(job.result.headings).map(([tag, count]) => (
                    <span key={tag} className="inline-block mr-3 text-xs">
                      {tag}: {count}
                    </span>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      <div className="mt-4">
        <button
          onClick={() => setExpanded(prev => !prev)}
          className="text-sm font-medium mb-2 flex items-center"
        >
          <span className="mr-1">{expanded ? '▼' : '▶'}</span>
          <span>Tasks</span>
        </button>
        
        {expanded && (
          <div className="mt-2">
            <TaskCard jobId={job.id} jobStatus={job.status} />
          </div>
        )}
      </div>
    </div>
  );
} 