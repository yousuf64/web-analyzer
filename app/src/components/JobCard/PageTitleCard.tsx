import { PageTitleIcon } from "./Icons";

export const PageTitleCard = ({ title }: { title: string }) => (
  <div className="flex items-center p-3 -mt-2 mb-3">
    <PageTitleIcon />
    <div className="ml-4 min-w-0">
      <p className="text-sm font-medium text-gray-500">Page Title</p>
      <p className="text-lg font-semibold text-gray-900 leading-tight truncate" title={title}>{title}</p>
    </div>
  </div>
);