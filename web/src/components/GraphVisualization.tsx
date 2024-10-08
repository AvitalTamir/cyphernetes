import React, { useCallback, useMemo } from 'react';
import { ForceGraph2D } from 'react-force-graph';
import './GraphVisualization.css';

interface GraphData {
  Nodes: {
    Id: string;
    Kind: string;
    Name: string;
    Namespace: string;
  }[];
  Edges: {
    From: string;
    To: string;
    Type: string;
  }[];
}

interface GraphVisualizationProps {
  data: string | null;
}

const GraphVisualization: React.FC<GraphVisualizationProps> = ({ data }) => {
  const graphData = useMemo(() => {
    if (!data) return { nodes: [], links: [] };

    const parsedData: GraphData = JSON.parse(data);
    
    // Create a map to store unique nodes
    const nodeMap = new Map();
    parsedData.Nodes.forEach(node => {
      const id = `${node.Kind}/${node.Name}`;
      if (!nodeMap.has(id)) {
        nodeMap.set(id, {
          id,
          kind: node.Kind,
          name: node.Name,
          namespace: node.Namespace
        });
      }
    });

    const nodes = Array.from(nodeMap.values());
    const links = parsedData.Edges.map(edge => ({
      source: edge.From,
      target: edge.To,
      type: edge.Type
    }));

    return { nodes, links };
  }, [data]);

  const nodeCanvasObject = useCallback((node: any, ctx: CanvasRenderingContext2D, globalScale: number) => {
    const label = node.name;
    const fontSize = 10/globalScale;
    ctx.font = `${fontSize}px Sans-Serif`;
    const textWidth = ctx.measureText(label).width;
    const nodeRadius = 15;

    // Draw circle
    ctx.beginPath();
    ctx.arc(node.x, node.y, nodeRadius, 0, 2 * Math.PI);
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    ctx.fill();

    // Draw text
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillStyle = node.color || '#000000';
    ctx.fillText(label, node.x, node.y);
  }, []);

  if (!data) return <div className="graph-visualization">No graph data available</div>;

  return (
    <div className="graph-visualization">
      <ForceGraph2D
        graphData={graphData}
        nodeLabel="name"
        nodeCanvasObject={nodeCanvasObject}
        nodeCanvasObjectMode={() => 'replace'}
        linkLabel="type"
        linkDirectionalArrowLength={3.5}
        linkDirectionalArrowRelPos={1}
        linkCurvature={0.25}
        linkDirectionalParticles={2}
        linkDirectionalParticleSpeed={0.005}
        // d3Force={('charge', null)}
        // d3Force={('link', null)}
        // d3Force={('center', null)}
        // d3Force={('collide', null)}
        // d3Force={('radial', null)}
        // d3Force={('x', null)}
        // d3Force={('y', null)}
        width={800}
        height={600}
      />
    </div>
  );
};

export default GraphVisualization;