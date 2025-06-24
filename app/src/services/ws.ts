import type { AnalyzeResult, JobStatus, TaskStatus, TaskType } from "../types";

interface JobUpdateMessage {
  type: 'job.update';
  job_id: string;
  status: JobStatus;
  result?: AnalyzeResult;
}

interface TaskStatusUpdateMessage {
  type: 'task.status_update';
  job_id: string;
  task_type: TaskType;
  status: TaskStatus;
}

interface SubTaskStatusUpdateMessage {
  type: 'task.subtask_status_update';
  job_id: string;
  task_type: TaskType;
  key: string;
  status: TaskStatus;
  url?: string;
}

type WebSocketMessage = JobUpdateMessage | TaskStatusUpdateMessage | SubTaskStatusUpdateMessage;

type JobUpdateCallback = (jobId: string, status: JobStatus, result?: AnalyzeResult) => void;
type TaskUpdateCallback = (jobId: string, taskType: TaskType, status: TaskStatus) => void;
type SubTaskUpdateCallback = (jobId: string, taskType: TaskType, key: string, status: TaskStatus, url?: string) => void;

const WS_URL = 'ws://localhost:8081/ws';

class WebSocketService {
  private ws: WebSocket | null = null;
  private isConnecting = false;
  private jobUpdateCallbacks: Set<JobUpdateCallback> = new Set();
  private taskUpdateCallbacks: Set<TaskUpdateCallback> = new Set();
  private subTaskUpdateCallbacks: Set<SubTaskUpdateCallback> = new Set();
  private subscribedJobIds: Set<string> = new Set();

  connect() {
    if (this.ws || this.isConnecting) {
      return;
    }

    this.isConnecting = true;
    this.ws = new WebSocket(WS_URL);

    this.ws.onopen = () => {
      this.isConnecting = false;
      console.log("WebSocket connection established.");
      
      // Re-subscribe to all job IDs after reconnection
      this.subscribedJobIds.forEach(jobId => {
        this.sendSubscriptionMessage('subscribe', jobId);
      });
    };

    this.ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data) as WebSocketMessage;

        switch (message.type) {
          case 'job.update':
            this.jobUpdateCallbacks.forEach(callback =>
              callback(message.job_id, message.status, message.result)
            );
            break;
          case 'task.status_update':
            this.taskUpdateCallbacks.forEach(callback =>
              callback(message.job_id, message.task_type, message.status)
            );
            break;
          case 'task.subtask_status_update':
            this.subTaskUpdateCallbacks.forEach(callback =>
              callback(message.job_id, message.task_type, message.key, message.status, message.url)
            );
            break;
          default:
            console.warn('Unknown message type:', message);
        }
      } catch (error) {
        console.error('Failed to parse WebSocket message:', error);
      }
    };

    this.ws.onclose = () => {
      this.ws = null;
      this.isConnecting = false;
      
      // Reconnect after a delay
      setTimeout(() => this.connect(), 5000);
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
      this.isConnecting = false;

      // Will trigger onclose and reconnect logic
      this.ws?.close();
    };
  }

  subscribeToJobUpdates(callback: JobUpdateCallback) {
    this.jobUpdateCallbacks.add(callback);
    return () => this.jobUpdateCallbacks.delete(callback);
  }

  subscribeToTaskUpdates(callback: TaskUpdateCallback) {
    this.taskUpdateCallbacks.add(callback);
    return () => this.taskUpdateCallbacks.delete(callback);
  }

  subscribeToSubTaskUpdates(callback: SubTaskUpdateCallback) {
    this.subTaskUpdateCallbacks.add(callback);
    return () => this.subTaskUpdateCallbacks.delete(callback);
  }

  subscribeToJob(jobId: string) {
    if (this.subscribedJobIds.has(jobId)) {
      return; // Already subscribed
    }

    this.subscribedJobIds.add(jobId);
    
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.sendSubscriptionMessage('subscribe', jobId);
    }
  }

  unsubscribeFromJob(jobId: string) {
    if (!this.subscribedJobIds.has(jobId)) {
      return; // Not subscribed
    }

    this.subscribedJobIds.delete(jobId);
    
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.sendSubscriptionMessage('unsubscribe', jobId);
    }
  }

  private sendSubscriptionMessage(action: 'subscribe' | 'unsubscribe', jobId: string) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      const message = {
        action,
        job_id: jobId
      };
      this.ws.send(JSON.stringify(message));
      console.log(`${action} to job ${jobId}`);
    }
  }
}

export const webSocketService = new WebSocketService(); 