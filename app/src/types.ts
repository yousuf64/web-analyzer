export interface Job {
  id: string;
  url: string;
  status: JobStatus;
  created_at: Date;
  updated_at: Date;
  started_at?: Date;
  completed_at?: Date;
  result?: AnalyzeResult;
}

export type JobStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';

export interface AnalyzeResult {
  html_version: string;
  page_title: string;
  headings: Record<string, number>;
  links: string[];
  internal_link_count: number;
  external_link_count: number;
  accessible_links: number;
  inaccessible_links: number;
  has_login_form: boolean;
}

export interface AnalyzeRequest {
  url: string;
}

export interface AnalyzeResponse {
  job: Job;
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
  url: string;
  description: string;
}

export type SubTaskType = 'validating_link'; 