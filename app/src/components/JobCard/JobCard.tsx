import { useState } from 'react';
import type { Job } from '../../types';
import { TaskCard } from '../TaskCard/TaskCard';
import { ProgressBar } from './ProgressBar';
import { StatusBadge } from './StatusBadge';
import { PageTitleCard } from './PageTitleCard';
import { HtmlVersionIcon, LoginFormIcon } from './Icons';
import { LinkAnalysisChart } from './LinkAnalysisChart';
import { HeadingsChart } from './HeadingChart';
import { StatCard } from './StatCard';

interface JobCardProps {
  job: Job;
  isNew?: boolean;
}

export function JobCard({ job, isNew = false }: JobCardProps) {
  const [expanded, setExpanded] = useState(isNew);

  return (
    <div className={`bg-white shadow-md border border-gray-200 rounded-lg transition-all duration-300 ${isNew ? 'ring-2 ring-blue-500' : ''}`}>
      <div
        className="flex justify-between items-center p-4 cursor-pointer"
        onClick={() => setExpanded(prev => !prev)}
      >
        <div className="flex-grow pr-4 min-w-0">
          <p
            className="text-sm font-medium text-gray-900 truncate"
            title={job.url}
          >
            {job.url}
          </p>
          <ProgressBar status={job.status} />
        </div>
        <div className="flex items-center space-x-4">
          <div className="w-24 flex justify-center">
            <StatusBadge status={job.status} />
          </div>
          <svg
            className={`w-6 h-6 text-gray-500 transform transition-transform duration-300 ${expanded ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            xmlns="http://www.w3.org/2000/svg"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 9l-7 7-7-7"></path>
          </svg>
        </div>
      </div>

      <div
        className={`overflow-hidden transition-all duration-500 ease-in-out ${expanded ? 'max-h-[2000px]' : 'max-h-0'}`}
      >
        <div className="border-t border-gray-200 p-4">
          {job.result && (
            <div className="pb-4">
              <h4 className="font-semibold mb-3 text-gray-800">Analysis Summary</h4>
              <PageTitleCard title={job.result.page_title} />
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <StatCard
                  icon={<HtmlVersionIcon />}
                  label="HTML Version"
                  value={job.result.html_version}
                />
                <StatCard
                  icon={<LoginFormIcon />}
                  label="Has Login Form"
                  value={job.result.has_login_form}
                />
                <LinkAnalysisChart
                  internal={job.result.internal_link_count}
                  external={job.result.external_link_count}
                  accessible={job.result.accessible_links}
                  inaccessible={job.result.inaccessible_links}
                />
                <HeadingsChart headings={job.result.headings} />
              </div>
            </div>
          )}

          {job.result && job.status !== 'pending' && (
            <hr className="my-4 border-gray-200" />
          )}

          {job.status !== 'pending' && (
            <div className="mt-2">
              <h4 className="font-semibold mb-2 text-gray-800">Task Progress</h4>
              <TaskCard jobId={job.id} jobStatus={job.status} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}