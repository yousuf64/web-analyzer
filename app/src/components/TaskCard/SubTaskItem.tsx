import type { SubTask } from "../../types";
import { StatusIcon } from "./StatusIcon";

export const SubTaskItem = ({ subtask }: { subtask: SubTask }) => (
    <div className="flex items-center space-x-3 py-1 pl-2 rounded-md hover:bg-white">
      <div className="flex-shrink-0">
        <StatusIcon status={subtask.status} />
      </div>
      <div className="flex-grow flex items-baseline min-w-0 text-sm">
        <span 
          className="font-mono text-gray-700 truncate"
          title={subtask.url}
        >
          {subtask.url}
        </span>
        {subtask.description && (
          <span className="ml-2 flex-shrink-0 px-2 py-0.5 text-xs font-medium bg-gray-200 text-gray-700 rounded-full">
            {subtask.description}
          </span>
        )}
      </div>
    </div>
  );