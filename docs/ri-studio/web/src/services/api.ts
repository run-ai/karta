import {
  ValidateResponse,
  ExtractResponse,
  ExamplesListResponse,
  ExampleResponse,
} from '../types';
import { wasmService } from './wasm';

/**
 * API service that uses WASM for all operations
 * This provides a static, server-free implementation
 */
export const api = {
  /**
   * Validate a Resource Interface YAML
   */
  async validateRI(ri: string): Promise<ValidateResponse> {
    return wasmService.validateRI(ri);
  },

  /**
   * Extract data from a Custom Resource using a Resource Interface
   */
  async extract(cr: string, ri: string): Promise<ExtractResponse> {
    return wasmService.extract(cr, ri);
  },

  /**
   * List all available example Resource Interfaces
   */
  async listExamples(): Promise<ExamplesListResponse> {
    return wasmService.listExamples();
  },

  /**
   * Get a specific example Resource Interface by name
   */
  async getExample(name: string): Promise<ExampleResponse> {
    return wasmService.getExample(name);
  },
};




