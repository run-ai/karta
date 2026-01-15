import { useState, useCallback, useMemo, useEffect } from 'react';
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

interface InstructionSet {
    nodeIds: string[];
    componentName: string;
    instanceId?: string;
    instructions: {
        topology?: 'rack' | 'zone' | 'region' | 'global';
        gpuAcceleration?: boolean;
    };
}

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
function RootNode({ data, selected }: { data: NodeData & { disabled?: boolean; hasInstructions?: boolean }; selected?: boolean }) {
    const disabled = data.disabled;
    const hasInstructions = data.hasInstructions;
    return (
        <>
            <div className={`px-4 py-3 border-2 rounded-lg shadow-lg min-w-[180px] text-center transition-all relative ${hasInstructions
                ? 'bg-blue-800 border-green-500 opacity-75 cursor-not-allowed'
                : disabled
                    ? 'bg-gray-600 border-gray-700 opacity-50 cursor-not-allowed'
                    : selected
                        ? 'bg-blue-600 border-yellow-400 ring-4 ring-yellow-400/50'
                        : 'bg-blue-600 border-blue-700'
                }`}>
                {hasInstructions && (
                    <div className="absolute top-1 right-1 text-green-400 text-lg">✓</div>
                )}
                <div className={`font-bold text-sm ${disabled || hasInstructions ? 'text-gray-300' : 'text-white'}`}>
                    {data.componentName}
                </div>
                {data.kind && (
                    <div className={`text-xs mt-1 ${hasInstructions ? 'text-gray-400' : disabled ? 'text-gray-500' : 'text-blue-200'}`}>
                        {data.kind.kind}
                    </div>
                )}
                <div className={`text-xs mt-1 font-semibold ${hasInstructions ? 'text-gray-400' : disabled ? 'text-gray-500' : 'text-blue-300'}`}>
                    Root Component
                </div>
            </div>
            <Handle type="source" position={Position.Bottom} />
        </>
    );
}

// Custom node component for Component nodes
function ComponentNode({ data, selected }: { data: NodeData & { disabled?: boolean; hasInstructions?: boolean }; selected?: boolean }) {
    const disabled = data.disabled;
    const hasInstructions = data.hasInstructions;
    return (
        <>
            <Handle type="target" position={Position.Top} />
            <div className={`px-4 py-3 border-2 rounded-lg shadow-lg min-w-[180px] text-center transition-all relative ${hasInstructions
                ? 'bg-green-800 border-green-500 opacity-75 cursor-not-allowed'
                : disabled
                    ? 'bg-gray-600 border-gray-700 opacity-50 cursor-not-allowed'
                    : selected
                        ? 'bg-green-600 border-yellow-400 ring-4 ring-yellow-400/50'
                        : 'bg-green-600 border-green-700'
                }`}>
                {hasInstructions && (
                    <div className="absolute top-1 right-1 text-green-400 text-lg">✓</div>
                )}
                <div className={`font-bold text-sm ${disabled || hasInstructions ? 'text-gray-300' : 'text-white'}`}>
                    {data.componentName}
                </div>
                {data.kind && (
                    <div className={`text-xs mt-1 ${hasInstructions ? 'text-gray-400' : disabled ? 'text-gray-500' : 'text-green-200'}`}>
                        {data.kind.kind}
                    </div>
                )}
                {data.instanceCount && (
                    <div className={`text-xs mt-1 ${hasInstructions ? 'text-gray-400' : disabled ? 'text-gray-500' : 'text-green-300'}`}>
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
function InstanceNode({ data, selected }: { data: NodeData & { disabled?: boolean; hasInstructions?: boolean }; selected?: boolean }) {
    const disabled = data.disabled;
    const hasInstructions = data.hasInstructions;
    return (
        <>
            <Handle type="target" position={Position.Top} />
            <div className={`px-4 py-3 border-2 rounded-lg shadow-lg min-w-[160px] text-center transition-all relative ${hasInstructions
                ? 'bg-orange-800 border-green-500 opacity-75 cursor-not-allowed'
                : disabled
                    ? 'bg-gray-600 border-gray-700 opacity-50 cursor-not-allowed'
                    : selected
                        ? 'bg-orange-600 border-yellow-400 ring-4 ring-yellow-400/50'
                        : 'bg-orange-600 border-orange-700'
                }`}>
                {hasInstructions && (
                    <div className="absolute top-1 right-1 text-green-400 text-lg">✓</div>
                )}
                <div className={`font-semibold text-sm ${disabled || hasInstructions ? 'text-gray-300' : 'text-white'}`}>
                    {data.instanceId}
                </div>
                <div className={`text-xs mt-1 ${hasInstructions ? 'text-gray-400' : disabled ? 'text-gray-500' : 'text-orange-200'}`}>
                    Instance
                </div>
            </div>
            <Handle type="source" position={Position.Bottom} />
        </>
    );
}

// Custom node component for Pod nodes
function PodNode({ data, selected }: { data: NodeData & { disabled?: boolean; hasInstructions?: boolean }; selected?: boolean }) {
    const disabled = data.disabled;
    const hasInstructions = data.hasInstructions;
    const scaleInfo = data.scale ? Object.values(data.scale)[0] : undefined;

    return (
        <>
            <Handle type="target" position={Position.Top} />
            <div className={`px-4 py-3 border-2 rounded-full shadow-lg min-w-[140px] text-center transition-all relative ${hasInstructions
                ? 'bg-purple-800 border-green-500 opacity-75 cursor-not-allowed'
                : disabled
                    ? 'bg-gray-600 border-gray-700 opacity-50 cursor-not-allowed'
                    : selected
                        ? 'bg-purple-600 border-yellow-400 ring-4 ring-yellow-400/50'
                        : 'bg-purple-600 border-purple-700'
                }`}>
                {hasInstructions && (
                    <div className="absolute top-1 right-1 text-green-400 text-lg">✓</div>
                )}
                <div className={`font-semibold text-sm ${disabled || hasInstructions ? 'text-gray-300' : 'text-white'}`}>
                    Pod(s)
                </div>
                {scaleInfo?.replicas && (
                    <div className={`text-xs mt-1 ${hasInstructions ? 'text-gray-400' : disabled ? 'text-gray-500' : 'text-purple-200'}`}>
                        replicas: {scaleInfo.replicas}
                    </div>
                )}
                {scaleInfo?.minReplicas && scaleInfo?.maxReplicas && (
                    <div className={`text-xs ${hasInstructions ? 'text-gray-400' : disabled ? 'text-gray-500' : 'text-purple-200'}`}>
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
    const [selectedNodeIds, setSelectedNodeIds] = useState<Set<string>>(new Set());
    const [viewDetailsNode, setViewDetailsNode] = useState<Node | null>(null);
    const [pendingInstructions, setPendingInstructions] = useState<{
        topology?: 'rack' | 'zone' | 'region' | 'global';
        gpuAcceleration?: boolean;
    }>({
        gpuAcceleration: false,
    });
    const [appliedInstructions, setAppliedInstructions] = useState<InstructionSet[]>([]);
    const [instructionsPanelHeight, setInstructionsPanelHeight] = useState(300);
    const [isInstructionsPanelCollapsed, setIsInstructionsPanelCollapsed] = useState(false);
    const [isResizing, setIsResizing] = useState(false);

    // Helper function to get all descendants of a node
    const getDescendants = useCallback((nodeId: string, edgesList: Edge[]): Set<string> => {
        const descendants = new Set<string>();
        const queue = [nodeId];

        while (queue.length > 0) {
            const currentId = queue.shift()!;
            const children = edgesList
                .filter(edge => edge.source === currentId)
                .map(edge => edge.target);

            children.forEach(childId => {
                if (!descendants.has(childId)) {
                    descendants.add(childId);
                    queue.push(childId);
                }
            });
        }

        return descendants;
    }, []);

    // Helper function to check if nodeA is a descendant of nodeB
    const isDescendantOf = useCallback((nodeId: string, ancestorId: string, edgesList: Edge[]): boolean => {
        const descendants = getDescendants(ancestorId, edgesList);
        return descendants.has(nodeId);
    }, [getDescendants]);

    // Get all node IDs that have instructions applied
    const nodesWithInstructions = useMemo(() => {
        const nodeIds = new Set<string>();
        appliedInstructions.forEach(instruction => {
            instruction.nodeIds.forEach(id => nodeIds.add(id));
        });
        return nodeIds;
    }, [appliedInstructions]);

    // Calculate all disabled nodes (descendants of selected nodes + nodes with instructions)
    const disabledNodeIds = useMemo(() => {
        const disabled = new Set<string>();

        // Add descendants of selected nodes
        selectedNodeIds.forEach(selectedId => {
            const descendants = getDescendants(selectedId, edges);
            descendants.forEach(descId => disabled.add(descId));
        });

        // Add nodes that already have instructions
        nodesWithInstructions.forEach(nodeId => disabled.add(nodeId));

        return disabled;
    }, [selectedNodeIds, edges, getDescendants, nodesWithInstructions]);

    const onNodesChange: OnNodesChange = useCallback(
        (changes) => setNodes((nds) => applyNodeChanges(changes, nds)),
        []
    );

    const onEdgesChange: OnEdgesChange = useCallback(
        (changes) => setEdges((eds) => applyEdgeChanges(changes, eds)),
        []
    );

    const onNodeClick = useCallback((event: React.MouseEvent, node: Node) => {
        event.stopPropagation();

        const isCtrlOrCmd = event.ctrlKey || event.metaKey;

        // Regular click: show node details
        if (!isCtrlOrCmd) {
            setViewDetailsNode(node);
            return;
        }

        // Ctrl/Cmd+Click: add to selection for instructions
        // Don't allow clicking on disabled nodes for instructions
        if (disabledNodeIds.has(node.id)) {
            return;
        }

        setSelectedNodeIds((prev) => {
            const newSet = new Set(prev);

            // Check if this node is a descendant of any selected node
            // or if any selected node is a descendant of this node
            for (const selectedId of prev) {
                if (isDescendantOf(node.id, selectedId, edges)) {
                    // This node is a descendant of a selected node, don't allow selection
                    return prev;
                }
                if (isDescendantOf(selectedId, node.id, edges)) {
                    // A selected node is a descendant of this node, remove that selected node
                    newSet.delete(selectedId);
                }
            }

            // Multi-select: toggle
            if (newSet.has(node.id)) {
                newSet.delete(node.id);
            } else {
                newSet.add(node.id);
            }
            return newSet;
        });
    }, [disabledNodeIds, edges, isDescendantOf]);

    const onPaneClick = useCallback(() => {
        setSelectedNodeIds(new Set());
        setViewDetailsNode(null);
    }, []);

    const handleApplyInstructions = useCallback(() => {
        if (selectedNodeIds.size === 0) return;
        if (!pendingInstructions.topology && !pendingInstructions.gpuAcceleration) return;

        // Collect all node IDs including descendants
        const allNodeIds = new Set<string>();
        selectedNodeIds.forEach(nodeId => {
            allNodeIds.add(nodeId);
            // Add all descendants of this node
            const descendants = getDescendants(nodeId, edges);
            descendants.forEach(descId => allNodeIds.add(descId));
        });

        const selectedNodes = nodes.filter(n => selectedNodeIds.has(n.id));
        const newInstructionSet: InstructionSet = {
            nodeIds: Array.from(allNodeIds),
            componentName: selectedNodes[0]?.data.componentName || 'unknown',
            instanceId: selectedNodes[0]?.data.instanceId,
            instructions: { ...pendingInstructions },
        };

        setAppliedInstructions((prev) => [...prev, newInstructionSet]);
        setSelectedNodeIds(new Set());
        setPendingInstructions({ gpuAcceleration: false });
    }, [selectedNodeIds, pendingInstructions, nodes, edges, getDescendants]);

    const handleClearAll = useCallback(() => {
        setAppliedInstructions([]);
        setSelectedNodeIds(new Set());
        setPendingInstructions({ gpuAcceleration: false });
    }, []);

    const handleMouseDown = useCallback((e: React.MouseEvent) => {
        e.preventDefault();
        setIsResizing(true);
    }, []);

    const handleMouseMove = useCallback((e: MouseEvent) => {
        if (!isResizing) return;
        const newHeight = window.innerHeight - e.clientY;
        if (newHeight >= 150 && newHeight <= window.innerHeight - 200) {
            setInstructionsPanelHeight(newHeight);
        }
    }, [isResizing]);

    const handleMouseUp = useCallback(() => {
        setIsResizing(false);
    }, []);

    // Add event listeners for resize
    useEffect(() => {
        if (isResizing) {
            document.addEventListener('mousemove', handleMouseMove);
            document.addEventListener('mouseup', handleMouseUp);
            return () => {
                document.removeEventListener('mousemove', handleMouseMove);
                document.removeEventListener('mouseup', handleMouseUp);
            };
        }
    }, [isResizing, handleMouseMove, handleMouseUp]);

    // Update nodes with selected, disabled, and hasInstructions state
    const nodesWithSelection = useMemo(() => {
        return nodes.map(node => ({
            ...node,
            selected: selectedNodeIds.has(node.id),
            data: {
                ...node.data,
                disabled: disabledNodeIds.has(node.id),
                hasInstructions: nodesWithInstructions.has(node.id),
            },
        }));
    }, [nodes, selectedNodeIds, disabledNodeIds, nodesWithInstructions]);

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
        <div className="flex flex-col h-full overflow-hidden">
            <div
                className="flex flex-1 min-h-0"
                style={{
                    height: appliedInstructions.length > 0 && !isInstructionsPanelCollapsed
                        ? `calc(100% - ${instructionsPanelHeight}px)`
                        : 'auto'
                }}
            >
                <div className="flex-1 relative">
                    <ReactFlow
                        nodes={nodesWithSelection}
                        edges={edges}
                        onNodesChange={onNodesChange}
                        onEdgesChange={onEdgesChange}
                        onNodeClick={onNodeClick}
                        onPaneClick={onPaneClick}
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
                                if (selectedNodeIds.has(node.id)) return '#facc15';
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

                    {/* Node Details Panel (regular click) */}
                    {viewDetailsNode && (
                        <div className="absolute top-4 left-4 bg-gray-800 border border-gray-700 rounded-lg p-4 shadow-xl max-w-md z-10 max-h-[calc(100%-2rem)] overflow-auto">
                            <div className="flex items-center justify-between mb-3">
                                <h3 className="text-white font-semibold">Node Details</h3>
                                <button
                                    onClick={() => setViewDetailsNode(null)}
                                    className="text-gray-400 hover:text-white"
                                >
                                    ✕
                                </button>
                            </div>

                            <div className="space-y-3 text-sm">
                                <div>
                                    <div className="text-gray-400 text-xs uppercase mb-1">Type</div>
                                    <div className="text-white">{viewDetailsNode.data.nodeType}</div>
                                </div>

                                <div>
                                    <div className="text-gray-400 text-xs uppercase mb-1">Name</div>
                                    <div className="text-white">{viewDetailsNode.data.componentName}</div>
                                </div>

                                {viewDetailsNode.data.instanceId && (
                                    <div>
                                        <div className="text-gray-400 text-xs uppercase mb-1">Instance ID</div>
                                        <div className="text-white">{viewDetailsNode.data.instanceId}</div>
                                    </div>
                                )}

                                {viewDetailsNode.data.kind && (
                                    <div>
                                        <div className="text-gray-400 text-xs uppercase mb-1">Kind</div>
                                        <div className="text-white">
                                            {viewDetailsNode.data.kind.group && `${viewDetailsNode.data.kind.group}/`}
                                            {viewDetailsNode.data.kind.version}/{viewDetailsNode.data.kind.kind}
                                        </div>
                                    </div>
                                )}

                                {viewDetailsNode.data.scale && (
                                    <div>
                                        <div className="text-gray-400 text-xs uppercase mb-1">Scale</div>
                                        <pre className="text-white text-xs bg-gray-900 p-2 rounded overflow-auto">
                                            {JSON.stringify(viewDetailsNode.data.scale, null, 2)}
                                        </pre>
                                    </div>
                                )}

                                {viewDetailsNode.data.instanceCount && (
                                    <div>
                                        <div className="text-gray-400 text-xs uppercase mb-1">Instances</div>
                                        <div className="text-white">{viewDetailsNode.data.instanceCount}</div>
                                    </div>
                                )}

                                {viewDetailsNode.data.error && (
                                    <div>
                                        <div className="text-red-400 text-xs uppercase mb-1">Error</div>
                                        <div className="text-red-300 text-xs">{viewDetailsNode.data.error}</div>
                                    </div>
                                )}

                                {/* Pod Template Spec */}
                                {viewDetailsNode.data.podTemplateSpec && (
                                    <div>
                                        <div className="text-gray-400 text-xs uppercase mb-1">Pod Template Spec</div>
                                        <pre className="text-green-400 text-xs bg-black p-2 rounded overflow-auto max-h-48">
                                            {extractSpecAsYaml(viewDetailsNode.data.podTemplateSpec)}
                                        </pre>
                                    </div>
                                )}

                                {/* Pod Spec */}
                                {viewDetailsNode.data.podSpec && (
                                    <div>
                                        <div className="text-gray-400 text-xs uppercase mb-1">Pod Spec</div>
                                        <pre className="text-green-400 text-xs bg-black p-2 rounded overflow-auto max-h-48">
                                            {extractSpecAsYaml(viewDetailsNode.data.podSpec)}
                                        </pre>
                                    </div>
                                )}

                                {/* Pod Metadata */}
                                {viewDetailsNode.data.podMetadata && (
                                    <div>
                                        <div className="text-gray-400 text-xs uppercase mb-1">Pod Metadata</div>
                                        <pre className="text-green-400 text-xs bg-black p-2 rounded overflow-auto max-h-48">
                                            {extractSpecAsYaml(viewDetailsNode.data.podMetadata)}
                                        </pre>
                                    </div>
                                )}

                                {/* Fragmented Spec */}
                                {viewDetailsNode.data.fragmentedSpec && (
                                    <div>
                                        <div className="text-gray-400 text-xs uppercase mb-1">Fragmented Spec</div>
                                        <pre className="text-green-400 text-xs bg-black p-2 rounded overflow-auto max-h-48">
                                            {JSON.stringify(viewDetailsNode.data.fragmentedSpec, null, 2)}
                                        </pre>
                                    </div>
                                )}
                            </div>
                        </div>
                    )}

                    {/* Instructions Panel (Ctrl/Cmd+Click) */}
                    {selectedNodeIds.size > 0 && (
                        <div className="absolute top-4 right-4 bg-gray-800 border border-gray-700 rounded-lg p-4 shadow-xl max-w-sm z-10">
                            <div className="flex items-center justify-between mb-3">
                                <h3 className="text-white font-semibold">
                                    Apply Instructions ({selectedNodeIds.size} node{selectedNodeIds.size > 1 ? 's' : ''})
                                </h3>
                                <button
                                    onClick={() => setSelectedNodeIds(new Set())}
                                    className="text-gray-400 hover:text-white"
                                >
                                    ✕
                                </button>
                            </div>

                            <div className="space-y-3 mb-4">
                                <div>
                                    <label className="block text-white text-sm mb-2">Topology</label>
                                    <select
                                        value={pendingInstructions.topology || ''}
                                        onChange={(e) => setPendingInstructions(prev => ({
                                            ...prev,
                                            topology: e.target.value ? e.target.value as 'rack' | 'zone' | 'region' | 'global' : undefined
                                        }))}
                                        className="w-full bg-gray-700 text-white border border-gray-600 rounded px-3 py-2 focus:outline-none focus:border-blue-500"
                                    >
                                        <option value="">None</option>
                                        <option value="rack">Rack</option>
                                        <option value="zone">Zone</option>
                                        <option value="region">Region</option>
                                        <option value="global">Global</option>
                                    </select>
                                </div>

                                <label className="flex items-center space-x-2 text-white cursor-pointer">
                                    <input
                                        type="checkbox"
                                        checked={pendingInstructions.gpuAcceleration || false}
                                        onChange={(e) => setPendingInstructions(prev => ({
                                            ...prev,
                                            gpuAcceleration: e.target.checked
                                        }))}
                                        className="w-4 h-4 rounded"
                                    />
                                    <span>GPU Acceleration (MNNVL)</span>
                                </label>
                            </div>

                            <button
                                onClick={handleApplyInstructions}
                                disabled={!pendingInstructions.topology && !pendingInstructions.gpuAcceleration}
                                className="w-full bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white px-4 py-2 rounded transition-colors"
                            >
                                Apply Instructions
                            </button>

                            <div className="mt-2 text-xs text-gray-400 space-y-1">
                                <div>• Click node: View details</div>
                                <div>• Ctrl/Cmd+Click: Select for instructions</div>
                            </div>
                        </div>
                    )}
                </div>
            </div>

            {/* Instructions JSON Output */}
            {appliedInstructions.length > 0 && (
                <div
                    className="border-t border-gray-700 bg-gray-900 flex flex-col"
                    style={{
                        height: isInstructionsPanelCollapsed ? 'auto' : `${instructionsPanelHeight}px`,
                        minHeight: isInstructionsPanelCollapsed ? 'auto' : '150px'
                    }}
                >
                    {/* Resize Handle */}
                    {!isInstructionsPanelCollapsed && (
                        <div
                            className="h-1 bg-gray-700 hover:bg-blue-500 cursor-ns-resize transition-colors"
                            onMouseDown={handleMouseDown}
                        />
                    )}

                    <div className="p-4 flex-1 flex flex-col min-h-0">
                        <div className="flex items-center justify-between mb-2">
                            <div className="flex items-center gap-2">
                                <h3 className="text-white font-semibold">Applied Instructions</h3>
                                <button
                                    onClick={() => setIsInstructionsPanelCollapsed(!isInstructionsPanelCollapsed)}
                                    className="text-gray-400 hover:text-white text-sm"
                                    title={isInstructionsPanelCollapsed ? "Expand" : "Collapse"}
                                >
                                    {isInstructionsPanelCollapsed ? '▲' : '▼'}
                                </button>
                            </div>
                            <button
                                onClick={handleClearAll}
                                className="text-sm text-red-400 hover:text-red-300"
                            >
                                Clear All
                            </button>
                        </div>

                        {!isInstructionsPanelCollapsed && (
                            <div className="bg-black rounded p-3 flex-1 overflow-auto">
                                <pre className="text-green-400 text-xs">
                                    {JSON.stringify(appliedInstructions, null, 2)}
                                </pre>
                            </div>
                        )}
                    </div>
                </div>
            )}
        </div>
    );
}
