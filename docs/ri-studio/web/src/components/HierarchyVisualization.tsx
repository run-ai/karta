import { useState, useCallback } from 'react';
import ReactFlow, {
    Background,
    Controls,
    MiniMap,
    Node,
    Edge,
    NodeTypes,
    OnNodesChange,
    OnEdgesChange,
    applyNodeChanges,
    applyEdgeChanges,
    Handle,
    Position,
} from 'reactflow';
import 'reactflow/dist/style.css';
import yaml from 'js-yaml';
import { ComponentResult } from '../types';
import { buildHierarchy } from '../utils/hierarchyBuilder';

interface HierarchyVisualizationProps {
    components: ComponentResult[];
}

interface NodeData {
    componentName: string;
    instanceId?: string;
    kind?: ComponentResult['kind'];
    scale?: ComponentResult['scale'];
    hasSpec: boolean;
    instanceCount?: number;
    error?: string;
    nodeType: 'root' | 'component' | 'instance' | 'pod';
    podTemplateSpec?: ComponentResult['podTemplateSpec'];
    podSpec?: ComponentResult['podSpec'];
    podMetadata?: ComponentResult['podMetadata'];
    fragmentedSpec?: ComponentResult['fragmentedSpec'];
}

// Helper function to extract spec value and convert to YAML
function extractSpecAsYaml(specData: Record<string, any> | undefined): string | null {
    if (!specData) return null;

    // Extract the actual spec value (unwrap the instance key)
    const values = Object.values(specData);
    if (values.length === 0) return null;

    const spec = values[0];

    try {
        return yaml.dump(spec, {
            indent: 2,
            lineWidth: 80,
            noRefs: true
        });
    } catch (error) {
        console.error('Failed to convert to YAML:', error);
        return null;
    }
}

// Custom node component for Root nodes
function RootNode({ data }: { data: NodeData }) {
    return (
        <>
            <div className="px-4 py-3 bg-blue-600 border-2 border-blue-700 rounded-lg shadow-lg min-w-[180px] text-center">
                <div className="text-white font-bold text-sm">{data.componentName}</div>
                {data.kind && (
                    <div className="text-blue-200 text-xs mt-1">
                        {data.kind.kind}
                    </div>
                )}
                <div className="text-blue-300 text-xs mt-1 font-semibold">Root Component</div>
            </div>
            <Handle type="source" position={Position.Bottom} />
        </>
    );
}

// Custom node component for Component nodes
function ComponentNode({ data }: { data: NodeData }) {
    return (
        <>
            <Handle type="target" position={Position.Top} />
            <div className="px-4 py-3 bg-green-600 border-2 border-green-700 rounded-lg shadow-lg min-w-[180px] text-center">
                <div className="text-white font-bold text-sm">{data.componentName}</div>
                {data.kind && (
                    <div className="text-green-200 text-xs mt-1">
                        {data.kind.kind}
                    </div>
                )}
                {data.instanceCount && (
                    <div className="text-green-300 text-xs mt-1">
                        {data.instanceCount} instance{data.instanceCount > 1 ? 's' : ''}
                    </div>
                )}
                {data.error && (
                    <div className="text-red-300 text-xs mt-1 font-semibold">⚠ Error</div>
                )}
            </div>
            <Handle type="source" position={Position.Bottom} />
        </>
    );
}

// Custom node component for Instance nodes
function InstanceNode({ data }: { data: NodeData }) {
    return (
        <>
            <Handle type="target" position={Position.Top} />
            <div className="px-4 py-3 bg-orange-600 border-2 border-orange-700 rounded-lg shadow-lg min-w-[160px] text-center">
                <div className="text-white font-semibold text-sm">{data.instanceId}</div>
                <div className="text-orange-200 text-xs mt-1">Instance</div>
            </div>
            <Handle type="source" position={Position.Bottom} />
        </>
    );
}

// Custom node component for Pod nodes
function PodNode({ data }: { data: NodeData }) {
    const scaleInfo = data.scale ? Object.values(data.scale)[0] : undefined;

    return (
        <>
            <Handle type="target" position={Position.Top} />
            <div className="px-4 py-3 bg-purple-600 border-2 border-purple-700 rounded-full shadow-lg min-w-[140px] text-center">
                <div className="text-white font-semibold text-sm">Pod(s)</div>
                {scaleInfo?.replicas && (
                    <div className="text-purple-200 text-xs mt-1">
                        replicas: {scaleInfo.replicas}
                    </div>
                )}
                {scaleInfo?.minReplicas && scaleInfo?.maxReplicas && (
                    <div className="text-purple-200 text-xs">
                        {scaleInfo.minReplicas}-{scaleInfo.maxReplicas}
                    </div>
                )}
            </div>
        </>
    );
}

const nodeTypes: NodeTypes = {
    root: RootNode,
    component: ComponentNode,
    instance: InstanceNode,
    pod: PodNode,
};

export function HierarchyVisualization({ components }: HierarchyVisualizationProps) {
    const { nodes: initialNodes, edges: initialEdges } = buildHierarchy(components);

    const [nodes, setNodes] = useState<Node[]>(initialNodes);
    const [edges, setEdges] = useState<Edge[]>(initialEdges);
    const [selectedNode, setSelectedNode] = useState<Node | null>(null);

    const onNodesChange: OnNodesChange = useCallback(
        (changes) => setNodes((nds) => applyNodeChanges(changes, nds)),
        []
    );

    const onEdgesChange: OnEdgesChange = useCallback(
        (changes) => setEdges((eds) => applyEdgeChanges(changes, eds)),
        []
    );

    const onNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
        setSelectedNode(node);
    }, []);

    if (!components || components.length === 0) {
        return (
            <div className="flex items-center justify-center h-full text-gray-400">
                No components to visualize. Run extraction first.
            </div>
        );
    }

    if (nodes.length === 0) {
        return (
            <div className="flex items-center justify-center h-full text-gray-400">
                Unable to build hierarchy. Check if components have proper relationships.
            </div>
        );
    }

    return (
        <div className="flex h-full">
            <div className="flex-1">
                <ReactFlow
                    nodes={nodes}
                    edges={edges}
                    onNodesChange={onNodesChange}
                    onEdgesChange={onEdgesChange}
                    onNodeClick={onNodeClick}
                    nodeTypes={nodeTypes}
                    nodesDraggable={false}
                    nodesConnectable={false}
                    elementsSelectable={true}
                    fitView
                    fitViewOptions={{
                        padding: 0.3,
                        includeHiddenNodes: false,
                        minZoom: 0.5,
                        maxZoom: 1.5,
                    }}
                    minZoom={0.1}
                    maxZoom={2}
                    defaultEdgeOptions={{
                        style: { stroke: '#9ca3af', strokeWidth: 2 },
                        type: 'smoothstep',
                    }}
                >
                    <Background color="#374151" gap={16} />
                    <Controls className="bg-gray-800 border-gray-700" />
                    <MiniMap
                        className="bg-gray-800 border-gray-700"
                        nodeColor={(node) => {
                            switch (node.type) {
                                case 'root': return '#2563eb';
                                case 'component': return '#16a34a';
                                case 'instance': return '#ea580c';
                                case 'pod': return '#9333ea';
                                default: return '#6b7280';
                            }
                        }}
                    />
                </ReactFlow>
            </div>

            {selectedNode && (
                <div className="w-80 bg-gray-800 border-l border-gray-700 p-4 overflow-auto">
                    <div className="flex items-center justify-between mb-4">
                        <h3 className="text-white font-semibold">Node Details</h3>
                        <button
                            onClick={() => setSelectedNode(null)}
                            className="text-gray-400 hover:text-white"
                        >
                            ✕
                        </button>
                    </div>

                    <div className="space-y-3">
                        <div>
                            <div className="text-gray-400 text-xs uppercase mb-1">Type</div>
                            <div className="text-white">{selectedNode.type}</div>
                        </div>

                        {selectedNode.data.componentName && (
                            <div>
                                <div className="text-gray-400 text-xs uppercase mb-1">Component</div>
                                <div className="text-white">{selectedNode.data.componentName}</div>
                            </div>
                        )}

                        {selectedNode.data.instanceId && (
                            <div>
                                <div className="text-gray-400 text-xs uppercase mb-1">Instance ID</div>
                                <div className="text-white">{selectedNode.data.instanceId}</div>
                            </div>
                        )}

                        {selectedNode.data.kind && (
                            <div>
                                <div className="text-gray-400 text-xs uppercase mb-1">Kind</div>
                                <div className="text-white">
                                    {selectedNode.data.kind.kind}
                                    <div className="text-gray-400 text-xs mt-1">
                                        {selectedNode.data.kind.group}/{selectedNode.data.kind.version}
                                    </div>
                                </div>
                            </div>
                        )}

                        {selectedNode.data.scale && (
                            <div>
                                <div className="text-gray-400 text-xs uppercase mb-1">Scale</div>
                                <div className="bg-gray-900 rounded p-2">
                                    <pre className="text-green-400 text-xs">
                                        {JSON.stringify(selectedNode.data.scale, null, 2)}
                                    </pre>
                                </div>
                            </div>
                        )}

                        {selectedNode.data.instanceCount && (
                            <div>
                                <div className="text-gray-400 text-xs uppercase mb-1">Instances</div>
                                <div className="text-white">{selectedNode.data.instanceCount}</div>
                            </div>
                        )}

                        <div>
                            <div className="text-gray-400 text-xs uppercase mb-1">Has Pod Spec</div>
                            <div className="text-white">
                                {selectedNode.data.hasSpec ? '✓ Yes' : '✗ No'}
                            </div>
                        </div>

                        {selectedNode.type === 'pod' && (
                            <>
                                {(() => {
                                    const podTemplateYaml = extractSpecAsYaml(selectedNode.data.podTemplateSpec);
                                    const podSpecYaml = extractSpecAsYaml(selectedNode.data.podSpec);
                                    const podMetadataYaml = extractSpecAsYaml(selectedNode.data.podMetadata);
                                    const fragmentedSpecYaml = extractSpecAsYaml(selectedNode.data.fragmentedSpec);

                                    return (
                                        <>
                                            {podTemplateYaml && (
                                                <div>
                                                    <div className="text-gray-400 text-xs uppercase mb-1">Pod Template Spec</div>
                                                    <div className="bg-gray-900 rounded p-2 max-h-60 overflow-auto">
                                                        <pre className="text-blue-400 text-xs whitespace-pre-wrap">
                                                            {podTemplateYaml}
                                                        </pre>
                                                    </div>
                                                </div>
                                            )}

                                            {podSpecYaml && (
                                                <div>
                                                    <div className="text-gray-400 text-xs uppercase mb-1">Pod Spec</div>
                                                    <div className="bg-gray-900 rounded p-2 max-h-60 overflow-auto">
                                                        <pre className="text-blue-400 text-xs whitespace-pre-wrap">
                                                            {podSpecYaml}
                                                        </pre>
                                                    </div>
                                                </div>
                                            )}

                                            {podMetadataYaml && (
                                                <div>
                                                    <div className="text-gray-400 text-xs uppercase mb-1">Pod Metadata</div>
                                                    <div className="bg-gray-900 rounded p-2 max-h-60 overflow-auto">
                                                        <pre className="text-blue-400 text-xs whitespace-pre-wrap">
                                                            {podMetadataYaml}
                                                        </pre>
                                                    </div>
                                                </div>
                                            )}

                                            {fragmentedSpecYaml && (
                                                <div>
                                                    <div className="text-gray-400 text-xs uppercase mb-1">Fragmented Spec</div>
                                                    <div className="bg-gray-900 rounded p-2 max-h-60 overflow-auto">
                                                        <pre className="text-purple-400 text-xs whitespace-pre-wrap">
                                                            {fragmentedSpecYaml}
                                                        </pre>
                                                    </div>
                                                </div>
                                            )}
                                        </>
                                    );
                                })()}
                            </>
                        )}

                        {selectedNode.data.error && (
                            <div>
                                <div className="text-red-400 text-xs uppercase mb-1">Error</div>
                                <div className="text-red-300 text-sm bg-red-900 rounded p-2">
                                    {selectedNode.data.error}
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            )}
        </div>
    );
}
