import React, { useRef, useCallback, useMemo } from 'react';
import ForceGraph3D from 'react-force-graph-3d';
import SpriteText from 'three-spritetext';
import './GraphVisualization.css';

interface Node {
  id: string;
  kind: string;
  name: string;
  namespace: string;
  group?: number;
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

  const graphData: GraphData = useMemo(() => {
    if (!data) return { nodes: [], links: [] };

    try {
      const parsedData = JSON.parse(data);
      const nodes = parsedData.Nodes.map((node: any, index: number) => ({
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

      console.log(nodes, links);

      return { nodes, links };
    } catch (error) {
      console.error("Failed to parse graph data:", error);
      return { nodes: [], links: [] };
    }
  }, [data]);

  //@ts-ignore
  const handleClick = useCallback(node => {
    // Aim at node from outside it
      const distance = 40;
      const distRatio = 1 + distance/Math.hypot(node.x, node.y, node.z);

      fgRef.current.cameraPosition(
        { x: node.x * distRatio, y: node.y * distRatio, z: node.z * distRatio }, // new position
        node, // lookAt ({ x, y, z })
        3000  // ms transition duration
      );
  }, [fgRef]);
  

  return (
    <div className="graph-visualization">
      <ForceGraph3D
      ref={fgRef}
      graphData={graphData}
      nodeLabel="id"
      nodeAutoColorBy="group"
      onNodeClick={handleClick}
      />
    </div>
  );
};

export default GraphVisualization;