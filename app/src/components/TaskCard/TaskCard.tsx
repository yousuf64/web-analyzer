import { useEffect, useState } from 'react';
import { ApiService } from '../../services/api';
import { webSocketService } from '../../services/ws';
import type { Task, TaskType } from '../../types';
import { StatusIcon } from './StatusIcon';
import { SubTaskItem } from './SubTaskItem';

interface TaskCardProps {
  jobId: string;
  jobStatus: string;
}

interface TasksState {
  tasks: Task[] | null;
  loading: boolean;
  error: string | null;
}

const getTaskOrder = (taskType: TaskType): number => {
  const order: Record<TaskType, number> = {
    'extracting': 0,
    'identifying_version': 1,
    'analyzing': 2,
    'verifying_links': 3,
  };
  return order[taskType] ?? 99;
}

const sortTasks = (tasks: Task[]): Task[] =>
  [...tasks].sort((a, b) => getTaskOrder(a.type) - getTaskOrder(b.type));

const formatTaskType = (taskType: TaskType): string => {
  switch (taskType) {
    case 'extracting':
      return 'Extracting';
    case 'identifying_version':
      return 'Identifying Version';
    case 'analyzing':
      return 'Analyzing';
    case 'verifying_links':
      return 'Verifying Links';
    default:
      return taskType;
  }
};

const TaskItem = ({ task }: { task: Task }) => {
  const [expanded, setExpanded] = useState(true);
  const subtasks = task.subtasks ? Object.values(task.subtasks) : [];
  const completedSubtasks = subtasks.filter(st => st.status === 'completed' || st.status === 'failed').length;
  const totalSubtasks = subtasks.length;
  const progress = totalSubtasks > 0 ? (completedSubtasks / totalSubtasks) * 100 : 0;
  const hasSubtasks = totalSubtasks > 0;

  return (
    <div className="rounded-md bg-gray-50">
      <div
        className={`flex items-center space-x-3 p-2 ${hasSubtasks ? 'cursor-pointer' : ''}`}
        onClick={() => hasSubtasks && setExpanded(prev => !prev)}
      >
        <div className="flex-shrink-0">
          <StatusIcon status={task.status} />
        </div>
        <div className="flex-grow">
          <p className="font-medium text-sm text-gray-800">{formatTaskType(task.type)}</p>
          {hasSubtasks && (
            <div>
              <div className="flex justify-between items-center text-xs text-gray-500 mt-1">
                <span>{task.status === 'completed' ? 'Complete' : 'In Progress'}</span>
                <span>{completedSubtasks} / {totalSubtasks}</span>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-1 mt-1">
                <div
                  className="bg-blue-600 h-1 rounded-full transition-all duration-500"
                  style={{ width: `${progress}%` }}
                ></div>
              </div>
            </div>
          )}
        </div>
        {hasSubtasks && (
          <div className="flex-shrink-0">
            <svg
              className={`w-5 h-5 text-gray-400 transform transition-transform duration-300 ${expanded ? 'rotate-180' : ''}`}
              fill="none" stroke="currentColor" viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 9l-7 7-7-7"></path>
            </svg>
          </div>
        )}
      </div>

      {hasSubtasks && (
         <div className={`overflow-hidden transition-all duration-300 ease-in-out ${expanded ? 'max-h-[500px]' : 'max-h-0'}`}>
            <div className="border-t border-gray-200 bg-gray-50/50 p-2">
              <div className="max-h-60 overflow-y-auto space-y-1 pr-2">
                {subtasks.map((subtask) => (
                  <SubTaskItem key={subtask.url} subtask={subtask} />
                ))}
              </div>
            </div>
         </div>
      )}
    </div>
  );
};

const TaskSkeletonLoader = () => (
  <div className="space-y-2 animate-pulse">
    {[...Array(3)].map((_, i) => (
      <div key={i} className="flex items-center space-x-3 p-2">
        <div className="h-5 w-5 bg-gray-300 rounded-full"></div>
        <div className="flex-grow space-y-2">
          <div className="h-4 bg-gray-300 rounded w-1/3"></div>
          <div className="h-2 bg-gray-300 rounded w-full"></div>
        </div>
      </div>
    ))}
  </div>
);


export function TaskCard({ jobId, jobStatus }: TaskCardProps) {
  const [state, setState] = useState<TasksState>({
    tasks: null,
    loading: false,
    error: null,
  });

  useEffect(() => {
    setState(prev => ({ ...prev, loading: true }));
    ApiService.getTasks(jobId)
      .then(tasks => setState(prev => ({ ...prev, tasks: sortTasks(tasks), loading: false })))
      .catch(error => setState(prev => ({ ...prev, error: error.message, loading: false })));
  }, [jobId]);

  useEffect(() => {
    webSocketService.connect();

    const isCompleted = jobStatus === 'completed' || jobStatus === 'failed';

    if (isCompleted) {
      return;
    }

    // Set up subscriptions for active jobs
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
      (updatedJobId, taskType, key, subtask) => {
        if (updatedJobId === jobId) {
          setState(prev => {
            if (!prev.tasks) return prev;

            return {
              ...prev,
              tasks: prev.tasks.map(task => {
                if (task.type === taskType) {
                  const updatedSubtasks = { ...task.subtasks, [key]: subtask };

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

    return () => {
      unsubscribeTask();
      unsubscribeSubTask();
      webSocketService.unsubscribeFromGroup(jobId);
    };
  }, [jobId, jobStatus]);


  if (state.loading) {
    return <TaskSkeletonLoader />;
  }

  if (state.error) {
    return <div className="text-sm text-red-600 bg-red-50 p-3 rounded-md">Error: {state.error}</div>;
  }

  if (!state.tasks || state.tasks.length === 0) {
    return <div className="text-sm text-gray-500">No tasks to display for this job.</div>;
  }

  return (
    <div className="space-y-2">
      {state.tasks.map(task => (
        <TaskItem key={task.type} task={task} />
      ))}
    </div>
  );
} 