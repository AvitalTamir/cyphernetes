import React, { useRef, useCallback, useMemo, useState, useEffect } from 'react';
import ForceGraph3D from 'react-force-graph-3d';
import SpriteText from 'three-spritetext';
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
        height: containerRef.current.clientHeight - 10
      });
    }
  }, []);

  useEffect(() => {
    updateDimensions();
    window.addEventListener('resize', updateDimensions);
    return () => window.removeEventListener('resize', updateDimensions);
  }, [updateDimensions]);

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

      console.log(nodes, links);

      return { nodes, links };
    } catch (error) {
      console.error("Failed to parse graph data:", error);
      return { nodes: [], links: [] };
    }
  }, [data]);

  //@ts-ignore
  const handleClick = useCallback((node) => {
    const distance = 40;
    const distRatio = 1 + distance/Math.hypot(node.x || 0, node.y || 0, node.z || 0);

    fgRef.current.cameraPosition(
      { x: (node.x || 0) * distRatio, y: (node.y || 0) * distRatio, z: (node.z || 0) * distRatio },
      node,
      3000
    );
  }, [fgRef]);

  return (
    <div ref={containerRef} className="graph-visualization-container">
      <div className="graph-visualization">
        <ForceGraph3D
          ref={fgRef}
          graphData={graphData}
          nodeLabel="id"
          nodeAutoColorBy="kind"
          onNodeClick={handleClick}
          linkAutoColorBy="type"
          dagLevelDistance={100}
          width={dimensions.width}
          height={dimensions.height - 10}
          backgroundColor="#2d2d2d"
          nodeThreeObjectExtend={true}
          nodeThreeObject={(node: any) => {
            const sprite = new SpriteText(node.name);
            sprite.color = '#ffffff';
            sprite.textHeight = 1.5;
            return sprite;
          }}
        />
      </div>
    </div>
  );
};

export default GraphVisualization;