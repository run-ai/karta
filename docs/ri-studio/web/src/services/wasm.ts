import {
  ValidateResponse,
  ExtractResponse,
  ExamplesListResponse,
  ExampleResponse,
  ErrorResponse,
} from '../types';

// Extend Window interface to include Go WASM API
declare global {
  interface Window {
    Go?: any;
    validateRI?: (riYAML: string) => any;
    extractData?: (crYAML: string, riYAML: string) => any;
    listExamples?: () => any;
    getExample?: (name: string) => any;
  }
}

class WasmService {
  private wasmReady: boolean = false;
  private wasmReadyPromise: Promise<void>;
  private wasmReadyResolve?: () => void;

  constructor() {
    this.wasmReadyPromise = new Promise((resolve) => {
      this.wasmReadyResolve = resolve;
    });
  }

  /**
   * Initialize the WASM module
   */
  async initialize(): Promise<void> {
    if (this.wasmReady) {
      return;
    }

    try {
      // Load the wasm_exec.js script
      await this.loadScript('/wasm_exec.js');

      if (!window.Go) {
        throw new Error('Go WASM runtime not loaded. wasm_exec.js may not have loaded correctly.');
      }

      // Initialize Go WASM runtime
      const go = new window.Go();

      // Load and instantiate the WASM module
      const response = await fetch('/wasm.wasm');
      if (!response.ok) {
        throw new Error(`Failed to fetch WASM module: ${response.statusText}`);
      }

      const wasmBuffer = await response.arrayBuffer();
      const result = await WebAssembly.instantiate(wasmBuffer, go.importObject);

      // Run the WASM module
      go.run(result.instance);

      // Wait a bit for WASM to fully initialize
      await new Promise(resolve => setTimeout(resolve, 100));

      // Verify that functions are available
      if (!window.validateRI || !window.extractData || !window.listExamples || !window.getExample) {
        throw new Error('WASM functions not registered. WASM initialization may have failed.');
      }

      this.wasmReady = true;
      this.wasmReadyResolve?.();

      console.log('WASM module initialized successfully');
    } catch (error) {
      console.error('Failed to initialize WASM:', error);
      throw new Error(`WASM initialization failed: ${error instanceof Error ? error.message : String(error)}`);
    }
  }

  /**
   * Load a script dynamically
   */
  private loadScript(src: string): Promise<void> {
    return new Promise((resolve, reject) => {
      const script = document.createElement('script');
      script.src = src;
      script.onload = () => resolve();
      script.onerror = () => reject(new Error(`Failed to load script: ${src}`));
      document.head.appendChild(script);
    });
  }

  /**
   * Ensure WASM is ready before calling functions
   */
  private async ensureReady(): Promise<void> {
    if (!this.wasmReady) {
      await this.wasmReadyPromise;
    }
  }

  /**
   * Validate a Resource Interface YAML
   */
  async validateRI(ri: string): Promise<ValidateResponse> {
    await this.ensureReady();

    if (!window.validateRI) {
      throw new Error('validateRI function not available in WASM');
    }

    try {
      const result = window.validateRI(ri);
      
      if (this.isErrorResponse(result)) {
        throw new Error(result.error);
      }

      return result as ValidateResponse;
    } catch (error) {
      console.error('WASM validateRI error:', error);
      throw error;
    }
  }

  /**
   * Extract data from a Custom Resource using a Resource Interface
   */
  async extract(cr: string, ri: string): Promise<ExtractResponse> {
    await this.ensureReady();

    if (!window.extractData) {
      throw new Error('extractData function not available in WASM');
    }

    try {
      const result = window.extractData(cr, ri);
      
      if (this.isErrorResponse(result)) {
        throw new Error(result.error);
      }

      return result as ExtractResponse;
    } catch (error) {
      console.error('WASM extractData error:', error);
      throw error;
    }
  }

  /**
   * List all available examples
   */
  async listExamples(): Promise<ExamplesListResponse> {
    await this.ensureReady();

    if (!window.listExamples) {
      throw new Error('listExamples function not available in WASM');
    }

    try {
      const result = window.listExamples();
      
      if (this.isErrorResponse(result)) {
        throw new Error(result.error);
      }

      return result as ExamplesListResponse;
    } catch (error) {
      console.error('WASM listExamples error:', error);
      throw error;
    }
  }

  /**
   * Get a specific example by name
   */
  async getExample(name: string): Promise<ExampleResponse> {
    await this.ensureReady();

    if (!window.getExample) {
      throw new Error('getExample function not available in WASM');
    }

    try {
      const result = window.getExample(name);
      
      if (this.isErrorResponse(result)) {
        throw new Error(result.error);
      }

      return result as ExampleResponse;
    } catch (error) {
      console.error('WASM getExample error:', error);
      throw error;
    }
  }

  /**
   * Check if a response is an error response
   */
  private isErrorResponse(response: any): response is ErrorResponse {
    return response && typeof response === 'object' && 'error' in response;
  }

  /**
   * Check if WASM is ready
   */
  isReady(): boolean {
    return this.wasmReady;
  }
}

// Export a singleton instance
export const wasmService = new WasmService();

// Initialize WASM as soon as the module is imported
wasmService.initialize().catch(error => {
  console.error('Auto-initialization of WASM failed:', error);
});
