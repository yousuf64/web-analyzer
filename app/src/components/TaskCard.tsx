import { useEffect, useState } from 'react';
import { ApiService } from '../services/api';
import { webSocketService } from '../services/ws';
import type { Task, TaskType } from '../types';

interface TaskCardProps {
  jobId: string;
}

interface TasksState {
  tasks: Task[] | null;
  loading: boolean;
  error: string | null;
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

export function TaskCard({ jobId }: TaskCardProps) {
  const [state, setState] = useState<TasksState>({
    tasks: null,
    loading: false,
    error: null,
  });

  useEffect(() => {
    webSocketService.connect();
    webSocketService.subscribeToGroup(jobId);

    const unsubscribeTask = webSocketService.subscribeToTaskUpdates((updatedJobId, taskType, status) => {
      if (updatedJobId === jobId) {
        setState(prev => {
          if (!prev.tasks) return prev;

          return {
            ...prev,
            tasks: prev.tasks.map(task => {
              if (task.type === taskType) {
                return { ...task, status: status };
              }
              return task;
            })
          };
        });
      }
    });

    const unsubscribeSubTask = webSocketService.subscribeToSubTaskUpdates(
      (updatedJobId, taskType, key, status, url) => {
        if (updatedJobId === jobId) {
          setState(prev => {
            if (!prev.tasks) return prev;

            return {
              ...prev,
              tasks: prev.tasks.map(task => {
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

                  return {
                    ...task,
                    subtasks: Object.fromEntries(Object.entries(updatedSubtasks).sort(([a], [b]) => a.localeCompare(b)))
                  };
                }
                return task;
              })
            };
          });
        }
      }
    );

    setState(prev => ({ ...prev, loading: true }));
    ApiService.getTasks(jobId)
      .then(tasks => setState(prev => ({ ...prev, tasks: sortTasks(tasks), loading: false })))
      .catch(error => setState(prev => ({ ...prev, error: error.message, loading: false })));

    return () => {
      unsubscribeTask();
      unsubscribeSubTask();
      webSocketService.unsubscribeFromGroup(jobId);
    };
  }, [jobId]);



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
  );
} 