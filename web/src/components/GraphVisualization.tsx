import React, { useRef, useState, useEffect, useCallback, forwardRef, useImperativeHandle, useMemo } from 'react';
import ForceGraph2D from 'react-force-graph-2d';
import './GraphVisualization.css';

interface Node {
  id: string;
  kind: string;
  name: string;
  namespace: string;
}

interface Link {
  source: string;
  target: string;
  type: string;
}

interface GraphData {
  nodes: Node[];
  links: Link[];
}

interface GraphVisualizationProps {
  data: string | null;
  onNodeHover: (highlightedNodes: Set<any>) => void;
}

const GraphVisualization = forwardRef<{ resetGraph: () => void }, GraphVisualizationProps>(({ data, onNodeHover }, ref) => {
  const fgRef = useRef<any>();
  const containerRef = useRef<HTMLDivElement>(null);
  const [dimensions, setDimensions] = useState({ width: 0, height: 0 });
  const [theme, setTheme] = useState<'dark' | 'light'>('dark');

  const updateDimensions = useCallback(() => {
    if (containerRef.current) {
      setDimensions({
        width: containerRef.current.clientWidth,
        height: containerRef.current.clientHeight,
      });
    }
  }, []);

  useEffect(() => {
    updateDimensions();
    window.addEventListener('resize', updateDimensions);
    return () => window.removeEventListener('resize', updateDimensions);
  }, [updateDimensions]);

  const NODE_R = 8;
  const graphData: GraphData = useMemo(() => {
    if (!data) return { nodes: [], links: [] };

    try {
      const parsedData = JSON.parse(data);
      const nodes = parsedData.Nodes.map((node: any) => ({
        id: `${node.Kind}/${node.Name}`,
        dataRefId: node.Id,
        kind: node.Kind,
        name: node.Name,
      }));

      const links = parsedData.Edges.map((edge: any) => ({
        source: edge.From,
        target: edge.To,
        type: edge.Type
      }));
      // Add neighbors and links to nodes
      //@ts-ignore
      nodes.forEach(node => {
        node.neighbors = [];
        node.links = [];
      });

      //@ts-ignore
      links.forEach(link => {
        //@ts-ignore
        const sourceNode = nodes.find(node => `${node.kind}/${node.name}` === link.source);
        //@ts-ignore
        const targetNode = nodes.find(node => `${node.kind}/${node.name}` === link.target);

        if (sourceNode && targetNode) {
          sourceNode.neighbors.push(targetNode);
          targetNode.neighbors.push(sourceNode);
          sourceNode.links.push(link);
          targetNode.links.push(link);
        }
      });

      return { nodes, links };
    } catch (error) {
      console.error("Failed to parse graph data:", error);
      return { nodes: [], links: [] };
    }
  }, [data]);

  // Define distinct colors for different node kinds
  const getNodeColor = useCallback((node: any) => {
    // Vibrant color palette for different node kinds
    const colorMap: {[key: string]: string} = {
      'Pod': '#4285F4',         // Google Blue
      'Service': '#34A853',     // Google Green
      'Deployment': '#FBBC05',  // Google Yellow
      'StatefulSet': '#EA4335', // Google Red
      'ConfigMap': '#8E44AD',   // Purple
      'Secret': '#F39C12',      // Orange
      'PersistentVolumeClaim': '#1ABC9C', // Turquoise
      'Ingress': '#E74C3C',     // Bright Red
      'Job': '#3498DB',         // Light Blue
      'CronJob': '#2ECC71',     // Emerald
      'Namespace': '#9B59B6',   // Amethyst
      'ReplicaSet': '#E67E22',  // Carrot
      'DaemonSet': '#16A085',   // Green Sea
      'Endpoint': '#2980B9',    // Belize Hole
      'Node': '#F1C40F',        // Sunflower
    };
    
    return colorMap[node.kind] || (theme === 'dark' ? '#5555aa' : '#aaaaaa');
  }, [theme]);

  const [highlightNodes, setHighlightNodes] = useState(new Set());
  const [highlightLinks, setHighlightLinks] = useState(new Set());
  const [hoverNode, setHoverNode] = useState(null);
  const [isHighlightLocked, setIsHighlightLocked] = useState(false);

  const handleNodeHover = useCallback((node: any) => {
    if (!isHighlightLocked) {
      let newHighlightNodes;
      if (node) {
        newHighlightNodes = new Set([node, ...(node.neighbors || [])]);
        setHighlightNodes(newHighlightNodes);
        setHighlightLinks(new Set(node.links || []));
      } else {
        newHighlightNodes = new Set();
        setHighlightNodes(newHighlightNodes);
        setHighlightLinks(new Set());
      }
      setHoverNode(node || null);
      onNodeHover(newHighlightNodes);
    }
  }, [onNodeHover, isHighlightLocked]);

  const handleLinkHover = useCallback((link: any) => {
    if (!isHighlightLocked) {
      if (link) {
        const newHighlightNodes = new Set([link.source, link.target]);
        setHighlightNodes(newHighlightNodes);
        setHighlightLinks(new Set([link]));
        onNodeHover(newHighlightNodes);
      } else {
        setHighlightLinks(new Set());
        setHighlightNodes(new Set());
        onNodeHover(new Set());
      }
    }
  }, [onNodeHover, isHighlightLocked]);

  const handleNodeClick = useCallback((node: any, event: any) => {
    if (node) {
      const newHighlightNodes = new Set([node, ...(node.neighbors || [])]);
      setHighlightNodes(newHighlightNodes);
      setHighlightLinks(new Set(node.links || []));
      setIsHighlightLocked(true);
      onNodeHover(newHighlightNodes);

      // Zoom in on the clicked node
      const distance = 40;
      const distRatio = 1 + distance/Math.hypot(node.x, node.y, node.z);

      fgRef.current.centerAt(node.x, node.y, 1000);
      fgRef.current.zoom(5, 1000);
    } else {
      setHighlightNodes(new Set());
      setHighlightLinks(new Set());
      setIsHighlightLocked(false);
      onNodeHover(new Set());
    }
  }, [onNodeHover, fgRef]);

  const handleCanvasClick = useCallback(() => {
    if (isHighlightLocked) {
      setHighlightNodes(new Set());
      setHighlightLinks(new Set());
      setIsHighlightLocked(false);
      onNodeHover(new Set());

      // Reset zoom and center
      fgRef.current.centerAt();
      fgRef.current.zoom(1, 1000);
    }
  }, [isHighlightLocked, onNodeHover, fgRef]);

  const nodeCanvasObject = useCallback((node: any, ctx: CanvasRenderingContext2D, globalScale: number) => {
    ctx.save();  // Save the current canvas state
    const isDarkTheme = theme === 'dark';
    const textColor = isDarkTheme ? 'white' : 'black';
    const labelBackgroundColor = isDarkTheme ? 'rgba(0, 0, 0, 0.8)' : 'rgba(255, 255, 255, 0.8)';
    
    // Set global alpha based on highlight state
    ctx.globalAlpha = highlightNodes.size > 0
      ? (highlightNodes.has(node) ? 1 : 0.05)
      : isDarkTheme ? 0.8 : 0.9;

    // Draw the node
    ctx.beginPath();
    ctx.arc(node.x, node.y, NODE_R, 0, 2 * Math.PI, false);
    
    // Create a radial gradient for 3D effect
    const baseColor = getNodeColor(node);  // Use our custom color function
    
    // Ensure coordinates are finite numbers
    const x = isFinite(node.x) ? node.x : 0;
    const y = isFinite(node.y) ? node.y : 0;
    
    try {
      // Adjust light source position for more pronounced 3D effect
      const gradient = ctx.createRadialGradient(
        x - NODE_R/2, y - NODE_R/2, 0,  // Light source position (more pronounced)
        x, y, NODE_R * 1.2  // Slightly larger radius for better edge shading
      );
      
      // Parse the base color to create lighter and darker versions
      let r = 100, g = 100, b = 100; // Default values
      
      if (baseColor && typeof baseColor === 'string') {
        if (baseColor.startsWith('#')) {
          const hex = baseColor.slice(1);
          r = parseInt(hex.slice(0, 2), 16) || r;
          g = parseInt(hex.slice(2, 4), 16) || g;
          b = parseInt(hex.slice(4, 6), 16) || b;
        } else if (baseColor.startsWith('rgb')) {
          const match = baseColor.match(/\d+/g);
          if (match && match.length >= 3) {
            r = parseInt(match[0]) || r;
            g = parseInt(match[1]) || g;
            b = parseInt(match[2]) || b;
          }
        }
      }
      
      // Create lighter and darker versions for gradient with more contrast
      const lighterColor = `rgba(${Math.min(r + 100, 255)}, ${Math.min(g + 100, 255)}, ${Math.min(b + 100, 255)}, 1)`;
      const darkerColor = `rgba(${Math.max(r - 70, 0)}, ${Math.max(g - 70, 0)}, ${Math.max(b - 70, 0)}, 1)`;
      
      // Add color stops to gradient
      gradient.addColorStop(0, lighterColor);
      gradient.addColorStop(0.5, baseColor);
      gradient.addColorStop(1, darkerColor);
      
      ctx.fillStyle = gradient;
    } catch (error) {
      // Fallback to solid color if gradient creation fails
      console.warn("Gradient creation failed, using solid color", error);
      ctx.fillStyle = baseColor;
    }
    
    ctx.fill();

    // Draw highlight ring if needed
    if (highlightNodes.has(node)) {
      ctx.beginPath();
      ctx.arc(node.x, node.y, NODE_R, 0, 2 * Math.PI, false);
      if (isHighlightLocked) {
        ctx.strokeStyle = "rgba(140, 82, 255, 0.7)";  // Purple from the gradient
        ctx.lineWidth = 3;  // Slightly thicker line for locked highlights
      } else {
        // Add glow effect for hover
        if (node === hoverNode) {
          // Pink glow for hovered node
          ctx.shadowColor = '#ff57e6';
          ctx.shadowBlur = 15;
          ctx.strokeStyle = 'rgba(255, 0, 217, 0.94)';
        } else {
          // Light blue for neighbors
          ctx.shadowColor = '#03a9f4';
          ctx.shadowBlur = 10;
          ctx.strokeStyle = 'rgba(3, 169, 244, 0.6)';
        }
        ctx.lineWidth = 2;
      }
      ctx.stroke();
      
      // Reset shadow after drawing
      ctx.shadowColor = 'transparent';
      ctx.shadowBlur = 0;
    }

    // Draw node label
    const label = node.kind.replace(/[^A-Z]/g, '');
    ctx.font = '5px Sans-Serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillStyle = textColor;
    ctx.fillText(label, node.x, node.y);

    // Draw full label if highlighted
    if (highlightNodes.has(node)) {
      const fullLabel = `(${node.kind}) ${node.name}`;
      const largerFontSize = 14 / globalScale;
      ctx.font = `${largerFontSize}px Sans-Serif`;
      const textWidth = ctx.measureText(fullLabel).width;
      const padding = 6 / globalScale;
      const boxWidth = textWidth + padding * 2;
      const boxHeight = largerFontSize + padding * 2;
      
      const labelX = node.x;
      const labelY = node.y + NODE_R + boxHeight / 2 + 3 / globalScale + 1.5;

      ctx.fillStyle = labelBackgroundColor;
      ctx.fillRect(labelX - boxWidth / 2, labelY - boxHeight / 2, boxWidth, boxHeight);
      ctx.fillStyle = textColor;
      ctx.fillText(fullLabel, labelX, labelY);
      ctx.globalAlpha = 1;
    } else {
        ctx.globalAlpha = isDarkTheme ? 0.05 : 0.1;
    }

    ctx.restore();  // Restore the canvas state
    
  }, [highlightNodes, hoverNode, isHighlightLocked, getNodeColor, theme]);

  const linkCanvasObject = useCallback((link: any, ctx: CanvasRenderingContext2D) => {
    ctx.save();  // Save the current canvas state
    const isDarkTheme = theme === 'dark';
    const linkBaseColor = isDarkTheme ? 'rgba(255, 255, 255, 0.6)' : 'rgba(0, 0, 0, 0.6)';
    const linkHighlightMultiplier = isDarkTheme ? 0.45 : 0.6;
    const linkFadeMultiplier = isDarkTheme ? 0.01 : 0.02;

    ctx.beginPath();
    ctx.moveTo(link.source.x, link.source.y);
    ctx.lineTo(link.target.x, link.target.y);
    ctx.strokeStyle = linkBaseColor;
    ctx.lineWidth = 1;
    
    // Set global alpha based on highlight state
    ctx.globalAlpha = highlightLinks.size > 0
      ? (highlightLinks.has(link) ? linkHighlightMultiplier : linkFadeMultiplier)
      : linkHighlightMultiplier;

    ctx.stroke();

    ctx.restore();  // Restore the canvas state
  }, [highlightLinks, theme]);

  const resetGraph = useCallback(() => {
    setHighlightNodes(new Set());
    setHighlightLinks(new Set());
    setHoverNode(null);
    setIsHighlightLocked(false);
    if (fgRef.current) {
      fgRef.current.centerAt();
      fgRef.current.zoom(1, 1000);
    }
  }, []);

  useImperativeHandle(ref, () => ({
    resetGraph
  }));

  const toggleTheme = () => {
    setTheme(prevTheme => (prevTheme === 'dark' ? 'light' : 'dark'));
  };

  return (
    <div ref={containerRef} className={`graph-visualization-container ${theme}-theme`}>
      <button onClick={toggleTheme} className="theme-toggle-button" title={theme === 'dark' ? 'Switch to Light Mode' : 'Switch to Dark Mode'}>
        {theme === 'dark' ? '☀️' : '🌙'}
      </button>
      <div className="graph-visualization" data-testid="graph-container">
        <ForceGraph2D
          ref={fgRef}
          graphData={graphData}
          nodeRelSize={NODE_R}
          autoPauseRedraw={false}
          linkWidth={(link: Link) => highlightLinks.has(link) ? 5 : 1}
          linkDirectionalParticles={4}
          linkDirectionalParticleWidth={(link: Link) => highlightLinks.has(link) ? 4 : 0}
          nodeCanvasObjectMode={() => 'after'}
          nodeCanvasObject={nodeCanvasObject}
          onNodeHover={handleNodeHover}
          onLinkHover={handleLinkHover}
          onNodeClick={handleNodeClick}
          onBackgroundClick={handleCanvasClick}
          linkCanvasObject={linkCanvasObject}
          linkCanvasObjectMode={() => 'after'}
          nodeAutoColorBy="kind"
          height={dimensions.height}
          width={dimensions.width}
          linkColor={theme === 'dark' ? "#ffffff" : "#000000"}
          backgroundColor={theme === 'dark' ? "rgb(18,18,18)" : "rgb(240, 240, 240)"}
          nodeLabel={""}
        />
      </div>
    </div>
  );
});

export default GraphVisualization;