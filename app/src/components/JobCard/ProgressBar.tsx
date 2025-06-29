import type { JobStatus } from "../../types";

export const ProgressBar = ({ status }: { status: JobStatus }) => {
  let progress = 0;
  if (status === 'running') progress = 50;
  else if (status === 'completed') progress = 100;
  else if (status === 'failed') progress = 100;

  const color = status === 'failed' ? 'bg-red-500' : 'bg-blue-500';

  return (
    <div className="w-full bg-gray-200 rounded-full h-1.5 dark:bg-gray-700 mt-2">
      <div
        className={`${color} h-1.5 rounded-full transition-all duration-500`}
        style={{ width: `${progress}%` }}
      ></div>
    </div>
  );
};