import type { Job } from '../types';
import { TaskCard } from './TaskCard';

interface JobCardProps {
  job: Job;
}

export function JobCard({ job }: JobCardProps) {
  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString();
  };

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
          {job.status}
        </span>
      </div>

      <div className="text-sm mb-4">
        <p>Created: {formatDate(job.created_at)}</p>
        {job.started_at && <p>Started: {formatDate(job.started_at)}</p>}
        {job.completed_at && <p>Completed: {formatDate(job.completed_at)}</p>}
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
        <TaskCard jobId={job.id} />
      </div>
    </div>
  );
} 