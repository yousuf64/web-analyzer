interface StatCardProps {
  icon: React.ReactNode;
  label: string;
  value: string | number | boolean;
}

export const StatCard = ({ icon, label, value }: StatCardProps) => {
  const displayValue = typeof value === 'boolean' ? (value ? 'Yes' : 'No') : value;
  return (
    <div className="flex items-center p-3 bg-gray-50 rounded-lg shadow-sm">
      {icon}
      <div className="ml-4">
        <p className="text-xl font-semibold text-gray-900">{displayValue}</p>
        <p className="text-sm font-medium text-gray-500">{label}</p>
      </div>
    </div>
  );
};