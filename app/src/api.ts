import type { Job, AnalyzeRequest, AnalyzeResponse } from './types';

const API_BASE_URL = 'http://localhost:8080';

export class ApiService {
  static async createAnalyzeJob(url: string): Promise<AnalyzeResponse> {
    const request: AnalyzeRequest = { url };

    const response = await fetch(`${API_BASE_URL}/analyze`, {
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
    const response = await fetch(`${API_BASE_URL}/jobs`);

    if (!response.ok) {
      throw new Error(`Failed to fetch jobs: ${response.statusText}`);
    }

    return response.json();
  }
} 