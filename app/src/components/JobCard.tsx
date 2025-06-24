import { useEffect, useState } from 'react';
import type { Job, Task } from '../types';
import { TaskCard } from './TaskCard';
import { ApiService } from '../services/api';
import { webSocketService } from '../services/ws';

interface JobCardProps {
  job: Job;
}

export function JobCard({ job }: JobCardProps) {
  const [tasks, setTasks] = useState<Task[] | null>(null);

  useEffect(() => {
    const unsubscribeTask = webSocketService.subscribeToTaskUpdates((jobId, taskType, status) => {
      if (jobId === job.id) {
        setTasks(prevTasks => {
          if (!prevTasks) return prevTasks;

          return prevTasks.map(task => {
            if (task.type === taskType) {
              return { ...task, status: status };
            }
            return task;
          });
        });
      }
    });

    const unsubscribeSubTask = webSocketService.subscribeToSubTaskUpdates(
      (jobId, taskType, key, status, url) => {
        if (jobId === job.id) {
          setTasks(prevTasks => {
            if (!prevTasks) return prevTasks;

            return prevTasks.map(task => {
              if (task.type === taskType) {
                const updatedSubtasks = { ...task.subtasks };

                if (!updatedSubtasks[key] && url) {
                  // New subtask
                  updatedSubtasks[key] = {
                    type: 'validating_link',
                    status: status,
                    url
                  };
                } else if (updatedSubtasks[key]) {
                  // Update existing subtask
                  updatedSubtasks[key] = {
                    ...updatedSubtasks[key],
                    status: status
                  };
                }

                return { ...task, subtasks: Object.fromEntries(Object.entries(updatedSubtasks).sort(([a], [b]) => a.localeCompare(b))) };
              }
              return task;
            });
          });
        }
      }
    );

    ApiService.getTasks(job.id)
      .then(fetchedTasks => setTasks(fetchedTasks))
      .catch(error => console.error('Failed to fetch tasks:', error));

    return () => {
      unsubscribeTask();
      unsubscribeSubTask();
    };
  }, [job.id]);

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
        <TaskCard jobId={job.id} tasks={tasks} />
      </div>
    </div>
  );
} 