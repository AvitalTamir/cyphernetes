import React, { useRef, useCallback, useMemo, useState, useEffect } from 'react';
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
}

const GraphVisualization: React.FC<GraphVisualizationProps> = ({ data }) => {
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

      console.log(nodes, links);

      return { nodes, links };
    } catch (error) {
      console.error("Failed to parse graph data:", error);
      return { nodes: [], links: [] };
    }
  }, [data]);

  const [highlightNodes, setHighlightNodes] = useState(new Set());
  const [highlightLinks, setHighlightLinks] = useState(new Set());
  const [hoverNode, setHoverNode] = useState(null);

  const handleNodeHover = useCallback((node: any) => {
    if (node) {
      setHighlightNodes(new Set([node, ...(node.neighbors || [])]));
      setHighlightLinks(new Set(node.links || []));
    } else {
      setHighlightNodes(new Set());
      setHighlightLinks(new Set());
    }
    setHoverNode(node || null);
  }, []);

  const handleLinkHover = useCallback((link: any) => {
    if (link) {
      setHighlightLinks(new Set([link]));
      setHighlightNodes(new Set([link.source, link.target]));
    } else {
      setHighlightLinks(new Set());
      setHighlightNodes(new Set());
    }
  }, []);

  const nodeCanvasObject = useCallback((node: any, ctx: CanvasRenderingContext2D, globalScale: number) => {
    // Draw the node
    ctx.beginPath();
    ctx.arc(node.x, node.y, NODE_R, 0, 2 * Math.PI, false);
    ctx.fillStyle = node.__color;  // Use the color assigned by nodeAutoColorBy
    ctx.fill();

    if (highlightNodes.has(node)) {
      // Draw ring around the node
      ctx.beginPath();
      ctx.arc(node.x, node.y, NODE_R, 0, 2 * Math.PI, false);
      ctx.strokeStyle = node === hoverNode ? 'red' : 'orange';
      ctx.lineWidth = 2;
      ctx.stroke();
    }

    const label = node.kind.replace(/[^A-Z]/g, '');
    const fontSize = 4;
    ctx.font = `${fontSize}px Sans-Serif`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillStyle = 'white';
    ctx.fillText(label, node.x, node.y);

    if (highlightNodes.has(node)) {
      const fullLabel = `${node.kind}: ${node.name}`;
      const largerFontSize = 12 / globalScale;
      ctx.font = `${largerFontSize}px Sans-Serif`;
      const textWidth = ctx.measureText(fullLabel).width;
      const padding = 4 / globalScale;
      const boxWidth = textWidth + padding * 2;
      const boxHeight = largerFontSize + padding * 2;
      
      // Position label below the node
      const labelX = node.x;
      const labelY = node.y + NODE_R + boxHeight / 2 + 2 / globalScale + 1.5;

      ctx.fillStyle = 'rgba(0, 0, 0, 0.8)';
      ctx.fillRect(labelX - boxWidth / 2, labelY - boxHeight / 2, boxWidth, boxHeight);
      ctx.fillStyle = 'white';
      ctx.fillText(fullLabel, labelX, labelY);
    }
  }, [highlightNodes, hoverNode]);

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
};

export default GraphVisualization;