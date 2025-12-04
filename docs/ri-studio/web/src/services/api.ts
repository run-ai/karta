import {
  ValidateRequest,
  ValidateResponse,
  ExtractRequest,
  ExtractResponse,
  ExamplesListResponse,
  ExampleResponse,
} from '../types';

const API_BASE = '/api';

async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(error.error || response.statusText);
  }
  return response.json();
}

export const api = {
  async validateRI(ri: string): Promise<ValidateResponse> {
    const request: ValidateRequest = { ri };
    const response = await fetch(`${API_BASE}/validate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    });
    return handleResponse<ValidateResponse>(response);
  },

  async extract(cr: string, ri: string): Promise<ExtractResponse> {
    const request: ExtractRequest = { cr, ri };
    const response = await fetch(`${API_BASE}/extract`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    });
    return handleResponse<ExtractResponse>(response);
  },

  async listExamples(): Promise<ExamplesListResponse> {
    const response = await fetch(`${API_BASE}/examples`);
    return handleResponse<ExamplesListResponse>(response);
  },

  async getExample(name: string): Promise<ExampleResponse> {
    const response = await fetch(`${API_BASE}/examples/${name}`);
    return handleResponse<ExampleResponse>(response);
  },
};




