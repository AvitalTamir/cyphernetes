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
    
    // Set global alpha based on highlight state
    ctx.globalAlpha = highlightNodes.size > 0
      ? (highlightNodes.has(node) ? 1 : 0.05)
      : 0.8;

    // Draw the node
    ctx.beginPath();
    ctx.arc(node.x, node.y, NODE_R, 0, 2 * Math.PI, false);
    ctx.fillStyle = node.__color;
    ctx.fill();

    // Draw highlight ring if needed
    if (highlightNodes.has(node)) {
      ctx.beginPath();
      ctx.arc(node.x, node.y, NODE_R, 0, 2 * Math.PI, false);
      if (isHighlightLocked) {
        ctx.strokeStyle = "rgba(255, 255, 255, 0.7)";  // Almost black for locked highlights
        ctx.lineWidth = 3;  // Slightly thicker line for locked highlights
      } else {
        ctx.strokeStyle = node === hoverNode ? 'red' : 'orange';
        ctx.lineWidth = 2;
      }
      ctx.stroke();
    }

    // Draw node label
    const label = node.kind.replace(/[^A-Z]/g, '');
    ctx.font = '5px Sans-Serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillStyle = 'white';
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

      ctx.fillStyle = 'rgba(0, 0, 0, 0.8)';
      ctx.fillRect(labelX - boxWidth / 2, labelY - boxHeight / 2, boxWidth, boxHeight);
      ctx.fillStyle = 'white';
      ctx.fillText(fullLabel, labelX, labelY);
      ctx.globalAlpha = 1;
    } else {
        ctx.globalAlpha = 0.05;
    }

    ctx.restore();  // Restore the canvas state
    
  }, [highlightNodes, hoverNode, isHighlightLocked]);

  const linkCanvasObject = useCallback((link: any, ctx: CanvasRenderingContext2D) => {
    ctx.save();  // Save the current canvas state

    ctx.beginPath();
    ctx.moveTo(link.source.x, link.source.y);
    ctx.lineTo(link.target.x, link.target.y);
    ctx.strokeStyle = 'rgba(0, 0, 0, 0.45)';
    ctx.lineWidth = 1;
    
    // Set global alpha based on highlight state
    ctx.globalAlpha = highlightLinks.size > 0
      ? (highlightLinks.has(link) ? 0.45 : 0.01)
      : 0.45;

    ctx.stroke();

    ctx.restore();  // Restore the canvas state
  }, [highlightLinks]);

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

  return (
    <div ref={containerRef} className="graph-visualization-container">
      <div className="graph-visualization">
        <ForceGraph2D
          ref={fgRef}
          graphData={graphData}
          nodeRelSize={NODE_R}
          autoPauseRedraw={false}
          linkWidth={link => highlightLinks.has(link) ? 5 : 1}
          linkDirectionalParticles={4}
          linkDirectionalParticleWidth={link => highlightLinks.has(link) ? 4 : 0}
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
          linkColor="#ffffff"
          backgroundColor="#efefef"
          nodeLabel={""}
        />
      </div>
    </div>
  );
});

export default GraphVisualization;