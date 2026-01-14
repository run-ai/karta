import { ComponentResult } from '../types';
import { Node, Edge } from 'reactflow';

interface HierarchyNodeData {
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

export function buildHierarchy(components: ComponentResult[]): {
    nodes: Node<HierarchyNodeData>[];
    edges: Edge[];
} {
    if (!components || components.length === 0) {
        return { nodes: [], edges: [] };
    }

    const nodes: Node<HierarchyNodeData>[] = [];
    const edges: Edge[] = [];

    // Find root component (no ownerRef)
    const rootComponent = components.find(c => !c.ownerRef);
    if (!rootComponent) {
        console.warn('No root component found');
        return { nodes, edges };
    }

    // Create a map of components by name for quick lookup
    const componentsByName = new Map<string, ComponentResult>();
    components.forEach(c => componentsByName.set(c.name, c));

    // Track node positions for top-down layout
    const VERTICAL_SPACING = 150;
    const HORIZONTAL_SPACING = 300;

    // Build the hierarchy tree (top-down)
    // Returns the x position of the created node
    function processComponent(
        component: ComponentResult,
        parentId: string | null,
        parentX: number,
        level: number
    ): number {
        const hasSpec = !!(
            component.podTemplateSpec ||
            component.podSpec ||
            component.fragmentedSpec
        );

        // Check if this component has instances (with non-empty IDs)
        const validInstanceIds = component.instanceIds?.filter(id => id && id.trim() !== '') || [];
        const hasInstances = validInstanceIds.length > 0;
        const yPos = level * VERTICAL_SPACING;

        if (!hasInstances) {
            // Component without instances - create single component node
            const componentNodeId = `component-${component.name}`;

            nodes.push({
                id: componentNodeId,
                type: parentId ? 'component' : 'root',
                position: { x: parentX, y: yPos },
                data: {
                    componentName: component.name,
                    kind: component.kind,
                    scale: component.scale,
                    hasSpec,
                    error: component.error,
                    nodeType: parentId ? 'component' : 'root',
                },
            });

            if (parentId) {
                edges.push({
                    id: `edge-${parentId}-${componentNodeId}`,
                    source: parentId,
                    target: componentNodeId,
                });
            }

            // If has spec definition, add pod node (directly below)
            if (hasSpec) {
                const podNodeId = `pod-${component.name}`;
                const podY = (level + 1) * VERTICAL_SPACING;

                nodes.push({
                    id: podNodeId,
                    type: 'pod',
                    position: { x: parentX, y: podY },
                    data: {
                        componentName: component.name,
                        scale: component.scale,
                        hasSpec: true,
                        nodeType: 'pod',
                        podTemplateSpec: component.podTemplateSpec,
                        podSpec: component.podSpec,
                        podMetadata: component.podMetadata,
                        fragmentedSpec: component.fragmentedSpec,
                    },
                });

                edges.push({
                    id: `edge-${componentNodeId}-${podNodeId}`,
                    source: componentNodeId,
                    target: podNodeId,
                });
            }

            // Process children that reference this component
            const children = components.filter(c => c.ownerRef === component.name);
            if (children.length > 0) {
                const nextLevel = level + (hasSpec ? 2 : 1);
                // Center children under this component
                children.forEach((child, index) => {
                    // Calculate x position for each child, centered under parent
                    const childX = parentX + ((index - (children.length - 1) / 2) * HORIZONTAL_SPACING);
                    processComponent(child, componentNodeId, childX, nextLevel);
                });
            }

            return parentX;

        } else {
            // Component with instances - create component node and instance nodes
            const componentNodeId = `component-${component.name}`;
            const instanceCount = validInstanceIds.length;

            // Component node is at parentX
            nodes.push({
                id: componentNodeId,
                type: parentId ? 'component' : 'root',
                position: { x: parentX, y: yPos },
                data: {
                    componentName: component.name,
                    kind: component.kind,
                    instanceCount,
                    hasSpec,
                    error: component.error,
                    nodeType: parentId ? 'component' : 'root',
                },
            });

            if (parentId) {
                edges.push({
                    id: `edge-${parentId}-${componentNodeId}`,
                    source: parentId,
                    target: componentNodeId,
                });
            }

            // Create instance nodes centered under the component
            validInstanceIds.forEach((instanceId, index) => {
                const instanceNodeId = `instance-${component.name}-${instanceId}`;
                // Center instances under parent
                const instanceX = parentX + ((index - (instanceCount - 1) / 2) * HORIZONTAL_SPACING);
                const instanceY = (level + 1) * VERTICAL_SPACING;

                nodes.push({
                    id: instanceNodeId,
                    type: 'instance',
                    position: { x: instanceX, y: instanceY },
                    data: {
                        componentName: component.name,
                        instanceId,
                        hasSpec,
                        nodeType: 'instance',
                    },
                });

                edges.push({
                    id: `edge-${componentNodeId}-${instanceNodeId}`,
                    source: componentNodeId,
                    target: instanceNodeId,
                });

                // If has spec definition, add pod node directly below this instance
                if (hasSpec) {
                    const podNodeId = `pod-${component.name}-${instanceId}`;
                    const podY = (level + 2) * VERTICAL_SPACING;
                    const scaleInfo = component.scale?.[instanceId];

                    nodes.push({
                        id: podNodeId,
                        type: 'pod',
                        position: { x: instanceX, y: podY },
                        data: {
                            componentName: component.name,
                            instanceId,
                            scale: scaleInfo ? { [instanceId]: scaleInfo } : undefined,
                            hasSpec: true,
                            nodeType: 'pod',
                            podTemplateSpec: component.podTemplateSpec?.[instanceId] ? { [instanceId]: component.podTemplateSpec[instanceId] } : undefined,
                            podSpec: component.podSpec?.[instanceId] ? { [instanceId]: component.podSpec[instanceId] } : undefined,
                            podMetadata: component.podMetadata?.[instanceId] ? { [instanceId]: component.podMetadata[instanceId] } : undefined,
                            fragmentedSpec: component.fragmentedSpec?.[instanceId] ? { [instanceId]: component.fragmentedSpec[instanceId] } : undefined,
                        },
                    });

                    edges.push({
                        id: `edge-${instanceNodeId}-${podNodeId}`,
                        source: instanceNodeId,
                        target: podNodeId,
                        type: 'default',
                        style: { stroke: '#9ca3af', strokeWidth: 2 },
                    });
                }
            });

            // Process children that reference this component
            const children = components.filter(c => c.ownerRef === component.name);
            if (children.length > 0) {
                const nextLevel = level + (hasSpec ? 3 : 2);
                // Center children under this component
                children.forEach((child, index) => {
                    const childX = parentX + ((index - (children.length - 1) / 2) * HORIZONTAL_SPACING);
                    processComponent(child, componentNodeId, childX, nextLevel);
                });
            }

            return parentX;
        }
    }

    // Start processing from root at x=0 (will be centered by React Flow)
    processComponent(rootComponent, null, 0, 0);

    // Normalize positions to ensure all x values are non-negative
    if (nodes.length > 0) {
        const minX = Math.min(...nodes.map(n => n.position.x));
        if (minX < 0) {
            nodes.forEach(node => {
                node.position.x -= minX;
            });
        }
    }

    return { nodes, edges };
}
