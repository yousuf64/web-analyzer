import type { Job, AnalyzeRequest, AnalyzeResponse, Task } from './types';

const BASE_URL = 'http://localhost:8080';

export class ApiService {
  static async createAnalyzeJob(url: string): Promise<AnalyzeResponse> {
    const request: AnalyzeRequest = { url };

    const response = await fetch(`${BASE_URL}/analyze`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(request),
    });

    if (!response.ok) {
      throw new Error(`Failed to create analyze job: ${response.statusText}`);
    }

    return response.json();
  }

  static async getJobs(): Promise<Job[]> {
    const response = await fetch(`${BASE_URL}/jobs`);

    if (!response.ok) {
      throw new Error(`Failed to fetch jobs: ${response.statusText}`);
    }

    return response.json();
  }

  static async getTasks(jobId: string): Promise<Task[]> {
    const response = await fetch(`${BASE_URL}/jobs/${jobId}/tasks`);

    if (!response.ok) {
      throw new Error(`Failed to fetch tasks: ${response.statusText}`);
    }

    return response.json();
  }
} 