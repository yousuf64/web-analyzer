import type { JobStatus } from "../../types";

export const StatusBadge = ({ status }: { status: JobStatus }) => {
  const baseClasses = 'px-2 py-1 text-xs font-semibold rounded-full';
  let specificClasses = '';

  switch (status) {
    case 'pending':
      specificClasses = 'bg-yellow-100 text-yellow-800';
      break;
    case 'running':
      specificClasses = 'bg-blue-100 text-blue-800';
      break;
    case 'completed':
      specificClasses = 'bg-green-100 text-green-800';
      break;
    case 'failed':
      specificClasses = 'bg-red-100 text-red-800';
      break;
    case 'cancelled':
      specificClasses = 'bg-gray-100 text-gray-800';
      break;
    default:
      specificClasses = 'bg-gray-100 text-gray-800';
  }

  return (
    <span className={`${baseClasses} ${specificClasses}`}>
      {status.replace('_', ' ').toUpperCase()}
    </span>
  );
};