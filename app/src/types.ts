export interface Job {
  id: string;
  url: string;
  status: JobStatus;
  created_at: string;
  updated_at: string;
  started_at?: string;
  completed_at?: string;
  result?: AnalyzeResult;
}

export type JobStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';

export interface AnalyzeResult {
  html_version: string;
  page_title: string;
  headings: Record<string, number>;
  links: string[];
  has_login_form: boolean;
}

export interface AnalyzeRequest {
  url: string;
}

export interface AnalyzeResponse {
  job_id: string;
}

export interface Task {
  job_id: string;
  type: TaskType;
  status: TaskStatus;
  subtasks: Record<string, SubTask>;
}

export type TaskType = 
  | 'extracting' 
  | 'identifying_version' 
  | 'analyzing' 
  | 'verifying_links';

export type TaskStatus = 
  | 'pending' 
  | 'running' 
  | 'completed' 
  | 'failed' 
  | 'skipped';

export interface SubTask {
  type: SubTaskType;
  status: TaskStatus;
  url?: string;
}

export type SubTaskType = 'validating_link'; 