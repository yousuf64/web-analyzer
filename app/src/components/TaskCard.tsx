import { useState } from 'react';
import type { Task, SubTask, TaskType } from '../types';
import { ApiService } from '../api';

interface TaskCardProps {
  jobId: string;
}

interface TasksState {
  tasks: Task[] | null;
  loading: boolean;
  error: string | null;
  expanded: boolean;
}

export function TaskCard({ jobId }: TaskCardProps) {
  const [state, setState] = useState<TasksState>({
    tasks: null,
    loading: false,
    error: null,
    expanded: false,
  });


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

  function orderTasks(tasks: Task[]): Task[] {
    return [...tasks].sort((a, b) => getTaskOrder(a.type) - getTaskOrder(b.type));
  }

  const handleToggleExpand = async () => {
    if (!state.expanded) {
      setState(prev => ({ ...prev, loading: true, error: null }));
      try {
        const tasks = await ApiService.getTasks(jobId);
        const orderedTasks = orderTasks(tasks);
        setState(prev => ({
          ...prev,
          tasks: orderedTasks,
          loading: false,
          expanded: true
        }));
      } catch (error) {
        setState(prev => ({
          ...prev,
          loading: false,
          error: error instanceof Error ? error.message : 'Ran into an error while fetching tasks'
        }));
      }
    } else {
      setState(prev => ({ ...prev, expanded: !prev.expanded }));
    }
  };

  return (
    <div className="border border-gray-300 mt-4">
      <button
        onClick={handleToggleExpand}
        className="w-full p-2 text-left bg-gray-100 hover:bg-gray-200"
      >
        <span className="font-medium">
          Tasks {state.tasks?.length}
        </span>
        <span className="ml-2">
          {state.loading ? '...' : state.expanded ? '▼' : '▶'}
        </span>
      </button>

      {state.expanded && (
        <div className="border-t border-gray-300">
          {state.error && (
            <div className="p-2 bg-red-100 text-red-600">
              {state.error}
            </div>
          )}

          {state.tasks && (
            <div>
              {state.tasks.map((task) => (
                <TaskItem key={task.type} task={task} />
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

interface TaskItemProps {
  task: Task;
}

function TaskItem({ task }: TaskItemProps) {
  const [subtasksExpanded, setSubtasksExpanded] = useState(false);
  const subtasks = Object.entries(task.subtasks || {});
  const hasSubtasks = subtasks.length > 0;

  function formatTaskType(type: TaskType): string {
    switch (type) {
      case 'extracting':
        return 'Extracting';
      case 'identifying_version':
        return 'Identifying HTTP Version';
      case 'analyzing':
        return 'Analyzing';
      case 'verifying_links':
        return 'Verifying Links';
      default:
        return 'Unknown';
    }
  }

  return (
    <div className="p-2 border-b border-gray-200">
      <div className="flex justify-between items-center">
        <div>
          <span className="text-sm font-medium">
            {formatTaskType(task.type)}
          </span>
          <span className="ml-2 text-sm">
            [{task.status}]
          </span>
        </div>

        {hasSubtasks && (
          <button
            onClick={() => setSubtasksExpanded(!subtasksExpanded)}
            className="text-sm text-blue-600 hover:text-blue-800"
          >
            Subtasks ({subtasks.length}) {subtasksExpanded ? '▼' : '▶'}
          </button>
        )}
      </div>

      {hasSubtasks && subtasksExpanded && (
        <div className="mt-2 ml-4 pl-2 border-l border-gray-300">
          {subtasks.map(([key, subtask]) => (
            <SubTaskItem key={key} name={key} subtask={subtask} />
          ))}
        </div>
      )}
    </div>
  );
}

interface SubTaskItemProps {
  name: string;
  subtask: SubTask;
}

function SubTaskItem({ name, subtask }: SubTaskItemProps) {
  return (
    <div className="text-sm py-1">
      <span>{name}</span>
      <span className="ml-2">[{subtask.status}]</span>
      {subtask.url && (
        <span className="ml-2 text-gray-600">
          - {subtask.url}
        </span>
      )}
    </div>
  );
} 