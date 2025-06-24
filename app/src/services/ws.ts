import type { AnalyzeResult, JobStatus, SubTask, TaskStatus, TaskType } from "../types";

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

interface SubTaskUpdateMessage {
  type: 'task.subtask_update';
  job_id: string;
  task_type: TaskType;
  key: string;
  subtask: SubTask;
}

type WebSocketMessage = JobUpdateMessage | TaskStatusUpdateMessage | SubTaskUpdateMessage;

type JobUpdateCallback = (jobId: string, status: JobStatus, result?: AnalyzeResult) => void;
type TaskUpdateCallback = (jobId: string, taskType: TaskType, status: TaskStatus) => void;
type SubTaskUpdateCallback = (jobId: string, taskType: TaskType, key: string, subtask: SubTask) => void;

const WS_URL = 'ws://localhost:8081/ws';

class WebSocketService {
  private ws: WebSocket | null = null;
  private isConnecting = false;
  private jobUpdateCallbacks: Set<JobUpdateCallback> = new Set();
  private taskUpdateCallbacks: Set<TaskUpdateCallback> = new Set();
  private subTaskUpdateCallbacks: Set<SubTaskUpdateCallback> = new Set();
  private subscribedGroups: Set<string> = new Set();

  connect() {
    if (this.ws || this.isConnecting) {
      return;
    }

    this.isConnecting = true;
    this.ws = new WebSocket(WS_URL);

    this.ws.onopen = () => {
      this.isConnecting = false;
      console.log("WebSocket connection established.");
      
      // Re-subscribe to all groups after reconnection
      this.subscribedGroups.forEach(group => {
        this.sendSubscriptionMessage('subscribe', group);
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
          case 'task.subtask_update':
            this.subTaskUpdateCallbacks.forEach(callback =>
              callback(message.job_id, message.task_type, message.key, message.subtask)
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
  
  subscribeToGroup(group: string) {
    if (this.subscribedGroups.has(group)) {
      return; // Already subscribed
    }

    this.subscribedGroups.add(group);
    
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.sendSubscriptionMessage('subscribe', group);
    }
  }

  unsubscribeFromGroup(group: string) {
    if (!this.subscribedGroups.has(group)) {
      return; // Not subscribed
    }

    this.subscribedGroups.delete(group);
    
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.sendSubscriptionMessage('unsubscribe', group);
    }
  }

  private sendSubscriptionMessage(action: 'subscribe' | 'unsubscribe', group: string) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      const message = {
        action,
        group
      };
      this.ws.send(JSON.stringify(message));
      console.log(`${action} to group ${group}`);
    }
  }
}

export const webSocketService = new WebSocketService(); 