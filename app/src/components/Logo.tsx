export default function Logo() {
    return (
        <div className={`flex items-center gap-3`}>
            <img src="/logo.svg" alt="Web Analyzer Logo" className="h-10 w-10" />
            <h1 className="text-2xl font-bold tracking-tight text-gray-800">
                Web Analyzer
            </h1>
        </div>
    );
} 