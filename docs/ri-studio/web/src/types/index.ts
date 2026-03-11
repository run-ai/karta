// API Request/Response Types

export interface ValidateRequest {
  ri: string;
}

export interface ValidateResponse {
  valid: boolean;
  errors?: string[];
}

export interface ExtractRequest {
  cr: string;
  ri: string;
}

export interface ExtractResponse {
  success: boolean;
  errors?: string[];
  components?: ComponentResult[];
}

export interface ComponentResult {
  name: string;
  kind?: GroupVersionKind;
  ownerRef?: string;
  podTemplateSpec?: Record<string, any>;
  podSpec?: Record<string, any>;
  podMetadata?: Record<string, any>;
  fragmentedSpec?: Record<string, FragmentedPodSpec>;
  scale?: Record<string, Scale>;
  instanceIds?: string[];
  error?: string;
}

export interface GroupVersionKind {
  group: string;
  version: string;
  kind: string;
}

export interface FragmentedPodSpec {
  schedulerName?: string;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
  resources?: any;
  resourceClaims?: any[];
  podAffinity?: any;
  nodeAffinity?: any;
  containers?: any[];
  container?: any;
  priorityClassName?: string;
  image?: string;
}

export interface Scale {
  replicas?: number;
  minReplicas?: number;
  maxReplicas?: number;
}

export interface ExamplesListResponse {
  examples: ExampleInfo[];
}

export interface ExampleInfo {
  name: string;
  displayName: string;
  description: string;
}

export interface ExampleResponse {
  name: string;
  cr?: string;
  ri: string;
}

export interface ErrorResponse {
  error: string;
}

// Hierarchy Visualization Types

export interface HierarchyNodeData {
  kind?: GroupVersionKind;
  scale?: Scale;
  hasSpec: boolean;
  instanceCount?: number;
  error?: string;
  componentName: string;
  instanceId?: string;
}

export interface HierarchyNode {
  id: string;
  type: 'root' | 'component' | 'instance' | 'pod';
  label: string;
  data: HierarchyNodeData;
}

export interface HierarchyEdge {
  id: string;
  source: string;
  target: string;
  label?: string;
}