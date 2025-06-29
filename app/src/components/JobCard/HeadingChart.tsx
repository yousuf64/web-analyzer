export const HeadingsChart = ({ headings }: { headings: Record<string, number> }) => {
  const hasHeadings = Object.keys(headings).length > 0;
  const sortedHeadings = hasHeadings ? Object.entries(headings).sort(([a], [b]) => a.localeCompare(b)) : [];
  const maxCount = hasHeadings ? Math.max(...Object.values(headings)) : 0;

  return (
    <div className="col-span-1 sm:col-span-2 p-4 bg-gray-50 rounded-lg shadow-sm">
      <h3 className="text-md font-semibold text-gray-800 mb-3">Heading Distribution</h3>
      {hasHeadings ? (
        <div className="space-y-2">
          {sortedHeadings.map(([tag, count]) => (
            <div key={tag} className="flex items-center text-sm">
              <span className="w-8 font-mono text-gray-500">{tag}</span>
              <div className="flex-grow flex items-center">
                <div className="w-full bg-gray-200 rounded-full h-4 mr-2">
                  <div
                    className="bg-indigo-400 h-4 rounded-full transition-all duration-500"
                    style={{ width: maxCount > 0 ? `${(count / maxCount) * 100}%` : '0%' }}
                  ></div>
                </div>
                <span className="w-8 font-semibold text-gray-800">{count}</span>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <p className="text-sm text-gray-500 text-center py-4">No headings were found on this page.</p>
      )}
    </div>
  );
};