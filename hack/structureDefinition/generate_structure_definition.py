#!/usr/bin/env python3
"""
Kai-bolt Structure Definition Generator
=====================================
Generates visual diagrams showing the component hierarchy from RID files.
"""

import argparse
import os
import sys
import subprocess
import venv
import glob
from pathlib import Path
from typing import Dict, List, Optional, Set, Tuple

# Import visualization libraries after venv setup
matplotlib = None
patches = None
nx = None
FancyBboxPatch = None
Rectangle = None

# Visual Constants
class VisualConfig:
    # Figure settings
    FIGURE_SIZE = (14, 10)
    DPI = 300
    
    # Component styling
    COMPONENT_WIDTH = 2.0
    COMPONENT_HEIGHT = 0.8
    
    # Colors
    ROOT_COLOR = 'lightblue'
    ROOT_EDGE_COLOR = 'darkblue'
    REFERENCE_COLOR = 'lightgray'
    REFERENCE_EDGE_COLOR = 'gray'
    CHILD_COLOR = 'white'
    CHILD_EDGE_COLOR = 'black'
    
    # Box styling
    BOX_PADDING = 0.1
    REFERENCE_LINE_STYLE = '--'
    REFERENCE_LINE_WIDTH = 2.5
    NORMAL_LINE_WIDTH = 1.5
    
    # Array visualization
    CASCADE_OFFSET = 0.12
    CASCADE_COUNT = 3
    CASCADE_ALPHA = 0.3
    
    # Layout spacing
    VERTICAL_SPACING = 3.0
    HORIZONTAL_SPACING = 3.0
    ARRAY_SPACING = 1.5
    EDGE_OFFSET = 0.5
    MIN_PADDING = 2.0
    PADDING_RATIO = 0.3
    
    # Arrow styling
    ARROW_MUTATION_SCALE = 25
    OWNERSHIP_ARROW_WIDTH = 2
    REFERENCE_ARROW_WIDTH = 2.5
    ARROW_COLOR_OWNERSHIP = 'black'
    ARROW_COLOR_REFERENCE = 'blue'
    
    # Legend and child kinds box
    LEGEND_WIDTH = 0.24
    LEGEND_X = 0.02
    CHILD_KINDS_WIDTH = 0.26
    CHILD_KINDS_X = 0.72
    BOX_Y_TOP = 0.96
    BOX_PADDING_Y = 0.06
    ITEM_HEIGHT = 0.035
    BOX_EDGE_COLOR = '#333333'
    BOX_FACE_COLOR = 'white'
    TEXT_COLOR_DARK = '#333333'
    TEXT_COLOR_MEDIUM = '#555555'


def setup_virtual_environment(script_dir: Path) -> Path:
    """Set up virtual environment and install dependencies."""
    venv_dir = script_dir / "venv"
    
    print("Setting up virtual environment...")
    if not venv_dir.exists():
        print("   Creating virtual environment")
        venv.create(venv_dir, with_pip=True)
        print("   ✓ Virtual environment created")
    else:
        print("   Virtual environment already exists")
    
    # Install dependencies
    pip_path = venv_dir / "bin" / "pip"
    requirements_path = script_dir / "requirements.txt"
    
    print("-> Installing dependencies")
    result = subprocess.run([
        str(pip_path), "install", "-r", str(requirements_path)
    ], capture_output=True, text=True)
    
    if result.returncode == 0:
        print("   ✓ Dependencies installed")
    else:
        print(f"   ✗ Failed to install dependencies: {result.stderr}")
        sys.exit(1)
    
    return venv_dir


def import_visualization_libraries():
    """Import visualization libraries after venv setup."""
    global matplotlib, patches, nx, FancyBboxPatch, Rectangle
    
    try:
        import matplotlib.pyplot as plt
        import matplotlib.patches as patches
        import networkx as nx
        from matplotlib.patches import FancyBboxPatch, Rectangle
        matplotlib = plt
        return True
    except ImportError as e:
        print(f"Failed to import visualization libraries: {e}")
        return False


def parse_rid_file(file_path: Path) -> Optional[Dict]:
    """Parse a RID YAML file and return its content."""
    try:
        import yaml  # Import here after venv setup
        with open(file_path, 'r') as f:
            return yaml.safe_load(f)
    except Exception as e:
        print(f"Error parsing {file_path}: {e}")
        return None


def extract_kind_name(kind_obj) -> str:
    """Extract just the kind name from a kind object and ensure PascalCase."""
    if isinstance(kind_obj, dict):
        kind_name = kind_obj.get('kind', 'Unknown')
    else:
        kind_name = str(kind_obj)
    
    # Ensure PascalCase (first letter uppercase, rest as-is for acronyms)
    if kind_name and not kind_name[0].isupper():
        kind_name = kind_name[0].upper() + kind_name[1:]
    
    return kind_name


def extract_kind_with_group(kind_obj) -> str:
    """Extract kind name with group for non-k8s native kinds."""
    if isinstance(kind_obj, dict):
        group = kind_obj.get('group', '')
        kind_name = kind_obj.get('kind', 'Unknown')
        
        # Ensure PascalCase
        if kind_name and not kind_name[0].isupper():
            kind_name = kind_name[0].upper() + kind_name[1:]
        
        # Include group for non-core Kubernetes kinds
        if group and group not in ['', 'apps', 'batch', 'extensions']:
            return f"{group}/{kind_name}"
        else:
            return kind_name
    else:
        kind_str = str(kind_obj)
        if kind_str and not kind_str[0].isupper():
            kind_str = kind_str[0].upper() + kind_str[1:]
        return kind_str


def is_array_component(spec_path: str) -> bool:
    """Check if a component represents an array (contains [])."""
    return '[]' in spec_path if spec_path else False


def extract_array_base(spec_path: str) -> str:
    """Extract the base path for array components."""
    if '[]' in spec_path:
        return spec_path.split('[]')[0]
    return spec_path


class ComponentAnalyzer:
    """Analyzes components and their relationships."""
    
    def __init__(self, components: Dict):
        self.components = components
    
    def get_root_components(self) -> List[str]:
        """Find root components (no owner)."""
        return [name for name, comp in self.components.items() 
                if not comp.get('ownerName')]
    
    def group_nodes_by_array(self, nodes: List[str]) -> List[List[str]]:
        """Group nodes that are part of the same array."""
        array_groups = {}
        single_nodes = []
        
        for node in nodes:
            comp = self.components.get(node, {})
            spec_path = comp.get('specPath', '')
            
            if is_array_component(spec_path):
                base_path = extract_array_base(spec_path)
                if base_path not in array_groups:
                    array_groups[base_path] = []
                array_groups[base_path].append(node)
            else:
                single_nodes.append(node)
        
        # Return grouped arrays and single nodes
        result = list(array_groups.values())
        for node in single_nodes:
            result.append([node])
        
        return result
    
    def get_legend_items(self, ownership: Dict, references: Dict) -> List[Tuple[str, str, str]]:
        """Determine what legend items to show based on components present."""
        legend_items = []
        
        # Check for component types
        has_root = any(not comp.get('ownerName') for comp in self.components.values())
        has_refs = any(comp.get('isReference', False) for comp in self.components.values())
        has_regular = any(comp.get('ownerName') and not comp.get('isReference', False) 
                         for comp in self.components.values())
        
        # Add component type legends
        if has_root:
            legend_items.append(('Root Component', VisualConfig.ROOT_COLOR, 'component'))
        if has_refs:
            legend_items.append(('Reference Component', VisualConfig.REFERENCE_COLOR, 'component'))
        if has_regular:
            legend_items.append(('Child Component', VisualConfig.CHILD_COLOR, 'component'))
        
        # Check for relationship types
        if ownership:
            legend_items.append(('Ownership', VisualConfig.ARROW_COLOR_OWNERSHIP, 'solid_arrow'))
        if references:
            legend_items.append(('Reference', VisualConfig.ARROW_COLOR_REFERENCE, 'dashed_arrow'))
        
        # Check for array components
        has_arrays = any(is_array_component(comp.get('specPath', '')) 
                        for comp in self.components.values())
        if has_arrays:
            legend_items.append(('Cascading Shapes (Array)', None, 'special'))
        
        return legend_items


class LayoutManager:
    """Manages the layout and positioning of components."""
    
    def __init__(self, components: Dict, ownership: Dict, references: Dict):
        self.components = components
        self.ownership = ownership
        self.references = references
        self.analyzer = ComponentAnalyzer(components)
    
    def create_hierarchical_layout(self, root: str) -> Dict[str, Tuple[float, float]]:
        """Create a properly centered hierarchical layout."""
        # Calculate levels using BFS from root
        levels = self._calculate_levels(root)
        
        # Group nodes by level
        level_groups = self._group_by_level(levels)
        
        # Position nodes with proper spacing
        return self._position_nodes(level_groups, levels)
    
    def _calculate_levels(self, root: str) -> Dict[str, int]:
        """Calculate the hierarchical levels for all components."""
        levels = {root: 0}
        queue = [root]
        
        while queue:
            current = queue.pop(0)
            current_level = levels[current]
            
            for child in self.ownership.get(current, []):
                if child not in levels:
                    levels[child] = current_level + 1
                    queue.append(child)
        
        # Add referenced components at the same level as their referencing component
        for source, targets in self.references.items():
            if source in levels:
                source_level = levels[source]
                for target in targets:
                    if target not in levels:
                        levels[target] = source_level
        
        # Add any remaining components as root level
        for comp_name in self.components.keys():
            if comp_name not in levels:
                levels[comp_name] = 0
        
        return levels
    
    def _group_by_level(self, levels: Dict[str, int]) -> Dict[int, List[str]]:
        """Group nodes by their hierarchical level."""
        level_groups = {}
        for node, level in levels.items():
            if level not in level_groups:
                level_groups[level] = []
            level_groups[level].append(node)
        return level_groups
    
    def _position_nodes(self, level_groups: Dict[int, List[str]], levels: Dict[str, int]) -> Dict[str, Tuple[float, float]]:
        """Position nodes within their levels with proper spacing."""
        pos = {}
        max_level = max(levels.values()) if levels else 0
        
        for level, nodes in level_groups.items():
            y = (max_level - level) * VisualConfig.VERTICAL_SPACING
            
            # Group nodes by arrays
            array_groups = self.analyzer.group_nodes_by_array(nodes)
            total_width = self._calculate_total_width(array_groups)
            
            # Start from center
            x_offset = -total_width / 2
            
            for group_nodes in array_groups:
                if len(group_nodes) > 1:
                    # Array group - closer spacing
                    group_width = (len(group_nodes) - 1) * VisualConfig.ARRAY_SPACING
                    start_x = x_offset + group_width / 2
                    
                    for i, node in enumerate(group_nodes):
                        pos[node] = (start_x - (i * VisualConfig.ARRAY_SPACING), y)
                    
                    x_offset += group_width + VisualConfig.HORIZONTAL_SPACING
                else:
                    # Single node
                    pos[group_nodes[0]] = (x_offset, y)
                    x_offset += VisualConfig.HORIZONTAL_SPACING
        
        return pos
    
    def _calculate_total_width(self, array_groups: List[List[str]]) -> float:
        """Calculate total width needed for all groups."""
        total_width = 0
        for group_nodes in array_groups:
            if len(group_nodes) > 1:
                total_width += (len(group_nodes) - 1) * VisualConfig.ARRAY_SPACING + VisualConfig.HORIZONTAL_SPACING
            else:
                total_width += VisualConfig.HORIZONTAL_SPACING
        return max(0, total_width - VisualConfig.HORIZONTAL_SPACING)  # Remove last spacing
    
    def center_and_scale_layout(self, pos: Dict[str, Tuple[float, float]]) -> Dict[str, Tuple[float, float]]:
        """Center the layout around origin."""
        if not pos:
            return pos
        
        # Get bounds
        x_coords = [x for x, y in pos.values()]
        y_coords = [y for x, y in pos.values()]
        
        if not x_coords:
            return pos
        
        # Center around origin
        x_center = (max(x_coords) + min(x_coords)) / 2
        y_center = (max(y_coords) + min(y_coords)) / 2
        
        return {node: (x - x_center, y - y_center) for node, (x, y) in pos.items()}


class ComponentRenderer:
    """Handles the visual rendering of components."""
    
    def __init__(self, ax, components: Dict):
        self.ax = ax
        self.components = components
    
    def draw_components(self, pos: Dict[str, Tuple[float, float]]):
        """Draw all component nodes with appropriate styling."""
        for name, (x, y) in pos.items():
            comp = self.components[name]
            self._draw_single_component(name, comp, x, y)
    
    def _draw_single_component(self, name: str, comp: Dict, x: float, y: float):
        """Draw a single component with proper styling."""
        # Determine component properties
        is_ref = comp.get('isReference', False)
        is_root = not comp.get('ownerName')
        is_array = is_array_component(comp.get('specPath', ''))
        
        # Get styling
        style = self._get_component_style(is_root, is_ref)
        
        # Draw component(s)
        if is_array:
            self._draw_array_component(name, comp, x, y, style)
        else:
            self._draw_regular_component(name, comp, x, y, style)
    
    def _get_component_style(self, is_root: bool, is_ref: bool) -> Dict:
        """Get styling configuration for a component."""
        if is_root:
            return {
                'facecolor': VisualConfig.ROOT_COLOR,
                'edgecolor': VisualConfig.ROOT_EDGE_COLOR,
                'fontweight': 'bold',
                'linestyle': '-',
                'linewidth': VisualConfig.NORMAL_LINE_WIDTH
            }
        elif is_ref:
            return {
                'facecolor': VisualConfig.REFERENCE_COLOR,
                'edgecolor': VisualConfig.REFERENCE_EDGE_COLOR,
                'fontweight': 'normal',
                'linestyle': VisualConfig.REFERENCE_LINE_STYLE,
                'linewidth': VisualConfig.REFERENCE_LINE_WIDTH
            }
        else:
            return {
                'facecolor': VisualConfig.CHILD_COLOR,
                'edgecolor': VisualConfig.CHILD_EDGE_COLOR,
                'fontweight': 'normal',
                'linestyle': '-',
                'linewidth': VisualConfig.NORMAL_LINE_WIDTH
            }
    
    def _draw_array_component(self, name: str, comp: Dict, x: float, y: float, style: Dict):
        """Draw cascading boxes for array components."""
        for i in range(VisualConfig.CASCADE_COUNT):
            # Front box (i=0) at original position, others cascade behind/below
            offset_x = x - (i * VisualConfig.CASCADE_OFFSET)
            offset_y = y - (i * VisualConfig.CASCADE_OFFSET)
            
            # Front box is opaque, others are transparent
            alpha = 1.0 if i == 0 else VisualConfig.CASCADE_ALPHA
            
            box = FancyBboxPatch(
                (offset_x - VisualConfig.COMPONENT_WIDTH/2, offset_y - VisualConfig.COMPONENT_HEIGHT/2), 
                VisualConfig.COMPONENT_WIDTH, VisualConfig.COMPONENT_HEIGHT,
                boxstyle=f"round,pad={VisualConfig.BOX_PADDING}",
                facecolor=style['facecolor'],
                edgecolor=style['edgecolor'],
                linestyle=style['linestyle'],
                linewidth=style['linewidth'],
                alpha=alpha,
                zorder=5 - i  # Front box has highest z-order
            )
            self.ax.add_patch(box)
        
        # Put text on the front box position
        self._add_component_text(name, comp, x, y, style['fontweight'])
    
    def _draw_regular_component(self, name: str, comp: Dict, x: float, y: float, style: Dict):
        """Draw a single box for regular components."""
        box = FancyBboxPatch(
            (x - VisualConfig.COMPONENT_WIDTH/2, y - VisualConfig.COMPONENT_HEIGHT/2), 
            VisualConfig.COMPONENT_WIDTH, VisualConfig.COMPONENT_HEIGHT,
            boxstyle=f"round,pad={VisualConfig.BOX_PADDING}",
            facecolor=style['facecolor'],
            edgecolor=style['edgecolor'],
            linestyle=style['linestyle'],
            linewidth=style['linewidth'],
            zorder=3
        )
        self.ax.add_patch(box)
        
        self._add_component_text(name, comp, x, y, style['fontweight'])
    
    def _add_component_text(self, name: str, comp: Dict, x: float, y: float, fontweight: str):
        """Add text label to a component."""
        label = f"{name}\n({comp['kind']})"
        self.ax.text(x, y, label, ha='center', va='center', 
                   fontsize=10, fontweight=fontweight, zorder=10)


class EdgeRenderer:
    """Handles the rendering of edges between components."""
    
    def __init__(self, ax):
        self.ax = ax
    
    def draw_edges(self, G, pos: Dict[str, Tuple[float, float]]):
        """Draw all edges with appropriate styling."""
        for edge in G.edges(data=True):
            source, target, data = edge
            edge_type = data.get('type', 'ownership')
            
            if source not in pos or target not in pos:
                continue
            
            self._draw_single_edge(source, target, edge_type, pos)
    
    def _draw_single_edge(self, source: str, target: str, edge_type: str, pos: Dict):
        """Draw a single edge between components."""
        x1, y1 = pos[source]
        x2, y2 = pos[target]
        
        # Offset to avoid overlapping with boxes
        y1_offset = y1 - VisualConfig.EDGE_OFFSET
        y2_offset = y2 + VisualConfig.EDGE_OFFSET
        
        if edge_type == 'reference':
            arrow_props = {
                'arrowstyle': '->',
                'color': VisualConfig.ARROW_COLOR_REFERENCE,
                'linestyle': '--',
                'linewidth': VisualConfig.REFERENCE_ARROW_WIDTH,
                'mutation_scale': VisualConfig.ARROW_MUTATION_SCALE,
                'connectionstyle': "arc3,rad=0.0"
            }
        else:
            arrow_props = {
                'arrowstyle': '->',
                'color': VisualConfig.ARROW_COLOR_OWNERSHIP,
                'linewidth': VisualConfig.OWNERSHIP_ARROW_WIDTH,
                'mutation_scale': VisualConfig.ARROW_MUTATION_SCALE
            }
        
        self.ax.annotate('', xy=(x2, y2_offset), xytext=(x1, y1_offset),
                       arrowprops=arrow_props, zorder=1)


class UIElementRenderer:
    """Handles rendering of UI elements like legend and child kinds box."""
    
    def __init__(self, fig):
        self.fig = fig
    
    def draw_child_kinds_box(self, child_kinds: List):
        """Draw additional child kinds in a fixed position box."""
        if not child_kinds:
            return
        
        num_items = len(child_kinds)
        box_height = VisualConfig.BOX_PADDING_Y + (num_items * VisualConfig.ITEM_HEIGHT)
        box_y = VisualConfig.BOX_Y_TOP - box_height
        
        # Create box
        box = FancyBboxPatch(
            (VisualConfig.CHILD_KINDS_X, box_y), 
            VisualConfig.CHILD_KINDS_WIDTH, box_height,
            boxstyle="round,pad=0.005",
            facecolor=VisualConfig.BOX_FACE_COLOR,
            edgecolor=VisualConfig.BOX_EDGE_COLOR,
            linewidth=1.5,
            transform=self.fig.transFigure,
            zorder=20
        )
        self.fig.patches.append(box)
        
        # Add title
        title_x = VisualConfig.CHILD_KINDS_X + VisualConfig.CHILD_KINDS_WIDTH/2
        title_y = box_y + box_height - 0.008
        
        self.fig.text(title_x, title_y, 'Additional Child Kinds',
                ha='center', va='top', 
                fontsize=10, fontweight='bold', color=VisualConfig.TEXT_COLOR_DARK,
                transform=self.fig.transFigure, zorder=25)
        
        # Add items
        for i, child_kind in enumerate(child_kinds):
            kind_name = extract_kind_with_group(child_kind)
            item_x = VisualConfig.CHILD_KINDS_X + 0.015
            item_y = box_y + box_height - 0.035 - (i * VisualConfig.ITEM_HEIGHT)
            
            self.fig.text(item_x, item_y, f"• {kind_name}",
                    ha='left', va='top', 
                    fontsize=9, color=VisualConfig.TEXT_COLOR_MEDIUM,
                    transform=self.fig.transFigure, zorder=25)
    
    def draw_legend(self, legend_items: List[Tuple[str, str, str]]):
        """Draw legend explaining the visual elements."""
        if not legend_items:
            return
        
        legend_height = 0.04 + (len(legend_items) * VisualConfig.ITEM_HEIGHT)
        legend_y = VisualConfig.BOX_Y_TOP - legend_height
        
        # Create legend box
        legend_box = FancyBboxPatch(
            (VisualConfig.LEGEND_X, legend_y), 
            VisualConfig.LEGEND_WIDTH, legend_height,
            boxstyle="round,pad=0.005",
            facecolor=VisualConfig.BOX_FACE_COLOR,
            edgecolor=VisualConfig.BOX_EDGE_COLOR,
            linewidth=1.5,
            transform=self.fig.transFigure,
            zorder=20
        )
        self.fig.patches.append(legend_box)
        
        # Add legend title
        title_x = VisualConfig.LEGEND_X + VisualConfig.LEGEND_WIDTH/2
        title_y = legend_y + legend_height - 0.008
        self.fig.text(title_x, title_y, 'Legend',
                ha='center', va='top', 
                fontsize=10, fontweight='bold', color=VisualConfig.TEXT_COLOR_DARK,
                transform=self.fig.transFigure, zorder=25)
        
        # Add legend items
        for i, (label, color, item_type) in enumerate(legend_items):
            item_x = VisualConfig.LEGEND_X + 0.015
            item_y = legend_y + legend_height - 0.04 - (i * VisualConfig.ITEM_HEIGHT)
            
            self._draw_legend_item(label, color, item_type, item_x, item_y)
    
    def _draw_legend_item(self, label: str, color: str, item_type: str, x: float, y: float):
        """Draw a single legend item."""
        if item_type == 'component':
            # Draw small colored rectangle
            rect_width = 0.025
            rect_height = 0.015
            rect = Rectangle((x, y - rect_height/2), rect_width, rect_height,
                           facecolor=color, edgecolor=VisualConfig.BOX_EDGE_COLOR, linewidth=1,
                           transform=self.fig.transFigure, zorder=25)
            self.fig.patches.append(rect)
            
            self.fig.text(x + rect_width + 0.01, y, label,
                    ha='left', va='center', fontsize=9, color=VisualConfig.TEXT_COLOR_DARK,
                    transform=self.fig.transFigure, zorder=25)
        
        elif item_type in ['solid_arrow', 'dashed_arrow']:
            # Use text symbols for arrows
            symbol = "━━►" if item_type == 'solid_arrow' else "┅┅►"
            self.fig.text(x, y, f"{symbol} {label}",
                    ha='left', va='center', fontsize=9, color=color,
                    transform=self.fig.transFigure, zorder=25)
        
        elif item_type == 'special':
            # Array component description
            self.fig.text(x, y, label,
                    ha='left', va='center', fontsize=9, color=VisualConfig.TEXT_COLOR_DARK,
                    transform=self.fig.transFigure, zorder=25)


class StructureDefinitionGenerator:
    """Main generator class that coordinates the visualization creation."""
    
    def __init__(self):
        self.components = {}
        self.child_kinds = []
        self.references = {}
        self.ownership = {}
        self.framework_name = ""
    
    def parse_rid(self, rid_data: Dict):
        """Parse RID data and extract component structure."""
        spec = rid_data.get('spec', {})
        structure_def = spec.get('structureDefinition', {})
        
        # Extract framework name from metadata
        self.framework_name = rid_data.get('metadata', {}).get('name', 'unknown')
        
        # Extract components and build relationships
        self._extract_components(structure_def.get('components', []))
        self.child_kinds = structure_def.get('additionalChildKinds', [])
    
    def _extract_components(self, components_list: List[Dict]):
        """Extract components and build relationship mappings."""
        # First pass: extract all components
        for comp in components_list:
            name = comp.get('name')
            if name:
                self.components[name] = {
                    'kind': extract_kind_with_group(comp.get('kind')),
                    'specPath': comp.get('specPath', '.spec'),
                    'ownerName': comp.get('ownerName'),
                    'isReference': comp.get('isReference', False),
                    'references': comp.get('references', []),
                    'childSpecDefinition': comp.get('childSpecDefinition'),
                }
                
                # Build ownership relationships
                owner = comp.get('ownerName')
                if owner:
                    if owner not in self.ownership:
                        self.ownership[owner] = []
                    self.ownership[owner].append(name)
        
        # Second pass: build reference relationships
        for comp in components_list:
            name = comp.get('name')
            if name:
                for ref in comp.get('references', []):
                    ref_comp = ref.get('componentName')
                    if ref_comp and ref_comp in self.components:
                        if name not in self.references:
                            self.references[name] = []
                        self.references[name].append(ref_comp)
    
    def generate_image(self, output_path: Path) -> bool:
        """Generate PNG image using matplotlib + networkx."""
        try:
            if not import_visualization_libraries():
                return False
            
            # Create figure
            fig, ax = matplotlib.subplots(1, 1, figsize=VisualConfig.FIGURE_SIZE)
            ax.axis('off')
            
            # Create graph and layout
            G = self._create_graph()
            pos = self._create_layout(G)
            
            # Render all elements
            self._render_diagram(fig, ax, G, pos)
            
            # Save image
            self._save_image(fig, output_path)
            
            print(f"   ✓ Generated: {output_path}")
            return True
            
        except Exception as e:
            print(f"   ✗ Failed to generate {output_path}: {e}")
            import traceback
            traceback.print_exc()
            return False
    
    def _create_graph(self):
        """Create networkx graph with nodes and edges."""
        import networkx as nx
        
        G = nx.DiGraph()
        
        # Add nodes
        for name, comp in self.components.items():
            G.add_node(name, **comp)
        
        # Add edges
        for parent, children in self.ownership.items():
            for child in children:
                G.add_edge(parent, child, type='ownership')
        
        for source, targets in self.references.items():
            for target in targets:
                G.add_edge(source, target, type='reference')
        
        return G
    
    def _create_layout(self, G) -> Dict[str, Tuple[float, float]]:
        """Create hierarchical layout for components."""
        analyzer = ComponentAnalyzer(self.components)
        roots = analyzer.get_root_components()
        
        if roots and len(self.components) > 1:
            layout_manager = LayoutManager(self.components, self.ownership, self.references)
            pos = layout_manager.create_hierarchical_layout(roots[0])
            pos = layout_manager.center_and_scale_layout(pos)
        else:
            # Fallback for single component
            pos = {list(self.components.keys())[0]: (0, 0)}
        
        return pos
    
    def _render_diagram(self, fig, ax, G, pos: Dict[str, Tuple[float, float]]):
        """Render all diagram elements."""
        # Set axis limits
        self._set_axis_limits(ax, pos)
        
        # Render components and edges
        component_renderer = ComponentRenderer(ax, self.components)
        component_renderer.draw_components(pos)
        
        edge_renderer = EdgeRenderer(ax)
        edge_renderer.draw_edges(G, pos)
        
        # Render UI elements
        ui_renderer = UIElementRenderer(fig)
        
        if self.child_kinds:
            ui_renderer.draw_child_kinds_box(self.child_kinds)
        
        analyzer = ComponentAnalyzer(self.components)
        legend_items = analyzer.get_legend_items(self.ownership, self.references)
        ui_renderer.draw_legend(legend_items)
    
    def _set_axis_limits(self, ax, pos: Dict[str, Tuple[float, float]]):
        """Set proper axis limits with padding."""
        if not pos:
            ax.set_xlim(-2, 2)
            ax.set_ylim(-2, 2)
            return
        
        x_coords = [x for x, y in pos.values()]
        y_coords = [y for x, y in pos.values()]
        
        x_min, x_max = min(x_coords), max(x_coords)
        y_min, y_max = min(y_coords), max(y_coords)
        
        # Add padding
        padding_x = max(VisualConfig.MIN_PADDING, (x_max - x_min) * VisualConfig.PADDING_RATIO)
        padding_y = max(VisualConfig.MIN_PADDING, (y_max - y_min) * VisualConfig.PADDING_RATIO)
        
        ax.set_xlim(x_min - padding_x, x_max + padding_x)
        ax.set_ylim(y_min - padding_y, y_max + padding_y)
        ax.set_aspect('equal')
    
    def _save_image(self, fig, output_path: Path):
        """Save the generated image."""
        # Add title
        matplotlib.suptitle(f"{self.framework_name.title()} Structure Definition", 
                    fontsize=18, fontweight='bold', y=0.92)
        
        # Save with proper settings
        matplotlib.tight_layout()
        matplotlib.subplots_adjust(top=0.85, bottom=0.1, left=0.1, right=0.9)
        matplotlib.savefig(output_path, dpi=VisualConfig.DPI, bbox_inches='tight', 
                   facecolor='white', edgecolor='none', pad_inches=0.3)
        matplotlib.close()


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description='Generate structure definition diagrams from RID files',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python generate_structure_definition.py                          # Generate for all RIDs
  python generate_structure_definition.py --frameworks pytorch nimservice
  python generate_structure_definition.py --input-dir custom/rids --output-dir diagrams/
        """
    )
    
    parser.add_argument(
        '--frameworks', 
        nargs='*',
        help='List of framework names to process (without .yaml). If not specified, processes all YAML files in input directory.'
    )
    
    parser.add_argument(
        '--input-dir',
        type=Path,
        default=Path('docs/examples'),
        help='Directory containing RID YAML files (default: docs/examples)'
    )
    
    parser.add_argument(
        '--output-path',
        type=Path,
        help='Override output path entirely. If not specified, generates alongside RID files with pattern: <framework>-structure-definition.png'
    )
    
    args = parser.parse_args()
    
    # Setup environment
    script_dir = Path(__file__).parent
    venv_dir = setup_virtual_environment(script_dir)
    
    # Add virtual environment to Python path
    venv_site_packages = venv_dir / "lib" / f"python{sys.version_info.major}.{sys.version_info.minor}" / "site-packages"
    sys.path.insert(0, str(venv_site_packages))
    
    # Find and process RID files
    rid_files = _find_rid_files(args)
    if not rid_files:
        print("No RID files found to process.")
        return
    
    print(f"\nProcessing {len(rid_files)} RID files...")
    
    success_count = _process_rid_files(rid_files, args)
    
    # Print summary
    print(f"\n{'='*50}")
    print(f"Structure Definition Generation Complete!")
    print(f"Successfully generated: {success_count}/{len(rid_files)} diagrams")
    
    if success_count < len(rid_files):
        print("Some diagrams failed to generate. Check the output above for details.")
        sys.exit(1)


def _find_rid_files(args) -> List[Path]:
    """Find RID files to process based on arguments."""
    if args.frameworks:
        rid_files = [args.input_dir / f"{fw}.yaml" for fw in args.frameworks]
        return [f for f in rid_files if f.exists()]
    else:
        return list(args.input_dir.glob("*.yaml"))


def _process_rid_files(rid_files: List[Path], args) -> int:
    """Process all RID files and return success count."""
    success_count = 0
    
    for rid_file in rid_files:
        print(f"\n-> Processing {rid_file.name}")
        
        rid_data = parse_rid_file(rid_file)
        if not rid_data:
            continue
        
        # Generate diagram
        generator = StructureDefinitionGenerator()
        generator.parse_rid(rid_data)
        
        # Determine output path
        if args.output_path:
            output_path = args.output_path
        else:
            framework_name = rid_file.stem
            output_filename = f"{framework_name}-structure-definition.png"
            output_path = rid_file.parent / output_filename
        
        # Generate image
        if generator.generate_image(output_path):
            success_count += 1
    
    return success_count


if __name__ == "__main__":
    main() 