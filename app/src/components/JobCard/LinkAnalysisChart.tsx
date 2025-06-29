export const LinkAnalysisChart = ({ internal, external, accessible, inaccessible }: { internal: number; external: number; accessible: number; inaccessible: number; }) => {
  const total = internal + external;
  const internalPercent = total > 0 ? (internal / total) * 100 : 0;
  const circumference = 2 * Math.PI * 28; // 2 * pi * radius
  const internalStrokeDashoffset = circumference - (internalPercent / 100) * circumference;

  return (
    <div className="col-span-1 sm:col-span-2 p-4 bg-gray-50 rounded-lg shadow-sm">
      <h3 className="text-md font-semibold text-gray-800 mb-3">Link Analysis</h3>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 items-center">
        <div className="flex justify-center items-center">
          <svg className="w-40 h-40 transform -rotate-90" viewBox="0 0 64 64">
            <circle className="text-blue-100" strokeWidth="8" stroke="currentColor" fill="transparent" r="28" cx="32" cy="32" />
            <circle
              className="text-blue-500"
              strokeWidth="8"
              strokeDasharray={circumference}
              strokeDashoffset={internalStrokeDashoffset}
              strokeLinecap="round"
              stroke="currentColor"
              fill="transparent"
              r="28"
              cx="32"
              cy="32"
            />
            <text x="50%" y="50%" dominantBaseline="middle" textAnchor="middle" className="text-xl font-bold fill-current text-gray-800 transform rotate-90">
              {total}
            </text>
            <text x="50%" y="65%" dominantBaseline="middle" textAnchor="middle" className="text-xs fill-current text-gray-500 transform rotate-90">
              Total
            </text>
          </svg>
        </div>
        <div className="space-y-3 text-sm">
          <div className="flex items-center justify-between">
            <span className="flex items-center"><span className="w-3 h-3 rounded-full bg-blue-500 mr-2"></span>Internal</span>
            <span className="font-semibold">{internal}</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="flex items-center"><span className="w-3 h-3 rounded-full bg-blue-100 mr-2"></span>External</span>
            <span className="font-semibold">{external}</span>
          </div>
          <div className="border-t border-gray-200 my-2"></div>
          <div className="w-full bg-red-200 rounded-full h-2.5">
            <div 
              className="bg-green-500 h-2.5 rounded-full" 
              style={{ width: `${total > 0 ? (accessible / total) * 100 : 0}%`}}
              title={`Accessible: ${accessible}`}
            ></div>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-green-600">Accessible</span>
            <span className="font-semibold">{accessible}</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-red-600">Inaccessible</span>
            <span className="font-semibold">{inaccessible}</span>
          </div>
        </div>
      </div>
    </div>
  );
};