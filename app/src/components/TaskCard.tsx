import { useEffect, useState } from 'react';
import { ApiService } from '../services/api';
import type { Task, TaskType } from '../types';

interface TaskCardProps {
  jobId: string;
  tasks?: Task[] | null;
}

interface TasksState {
  tasks: Task[] | null;
  loading: boolean;
  error: string | null;
  expanded: boolean;
}

function getTaskOrder(taskType: TaskType): number {
  switch (taskType) {
    case 'extracting':
      return 0;
    case 'identifying_version':
      return 1;
    case 'analyzing':
      return 2;
    case 'verifying_links':
      return 3;
    default:
      throw new Error(`Unknown task type: ${taskType}`);
  }
}

function sortTasks(tasks: Task[]): Task[] {
  return [...tasks].sort((a, b) => getTaskOrder(a.type) - getTaskOrder(b.type));
}

export function TaskCard({ jobId, tasks: propTasks }: TaskCardProps) {
  const [state, setState] = useState<TasksState>({
    tasks: null,
    loading: false,
    error: null,
    expanded: false,
  });

  useEffect(() => {
    if (propTasks) {
      setState(prev => ({ ...prev, tasks: sortTasks(propTasks) }));
      return;
    }

    setState(prev => ({ ...prev, loading: true }));
    ApiService.getTasks(jobId)
      .then(tasks => setState(prev => ({ ...prev, tasks: sortTasks(tasks), loading: false })))
      .catch(error => setState(prev => ({ ...prev, error: error.message, loading: false })));
  }, [jobId, propTasks]);

  const handleToggleExpand = () => {
    setState(prev => ({ ...prev, expanded: !prev.expanded }));
  };

  if (state.loading) {
    return <div>Loading tasks...</div>;
  }

  if (state.error) {
    return <div>Error: {state.error}</div>;
  }

  if (!state.tasks || state.tasks.length === 0) {
    return <div>No tasks found</div>;
  }

  return (
    <div>
      <button
        onClick={handleToggleExpand}
        className="text-sm font-medium mb-2"
      >
        {state.expanded ? '▼' : '▶'} Tasks
      </button>

      {state.expanded && (
        <div className="space-y-2">
          {state.tasks.map(task => (
            <div key={task.type} className="border p-2">
              <div className="flex justify-between items-start">
                <div>
                  <span className="font-medium">{task.type}</span>
                  <span className="ml-2 text-sm">[{task.status}]</span>
                </div>
              </div>

              {task.subtasks && Object.keys(task.subtasks).length > 0 && (
                <div className="mt-2 ml-4 space-y-1">
                  {Object.entries(task.subtasks).map(([key, subTask]) => (
                    <div key={key} className="text-sm">
                      <span>{subTask.url}</span>
                      <span className="ml-2">[{subTask.status}]</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
} 