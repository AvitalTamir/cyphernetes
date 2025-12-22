import { type ReactNode, useEffect, useRef } from "react";
import clsx from "clsx";
import Link from "@docusaurus/Link";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";
import Layout from "@theme/Layout";
import CodeBlock from "@theme/CodeBlock";

import styles from "./index.module.css";

interface Orb {
  x: number;
  y: number;
  baseRadius: number;
  radius: number;
  color: string;
  vx: number;
  vy: number;
  phase: number;
  pulseSpeed: number;
}

function FloatingOrbs() {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const setCanvasSize = () => {
      const dpr = window.devicePixelRatio || 1;
      canvas.width = canvas.offsetWidth * dpr;
      canvas.height = canvas.offsetHeight * dpr;
      ctx.scale(dpr, dpr);
    };
    setCanvasSize();
    window.addEventListener("resize", setCanvasSize);

    // Create bolder, more dramatic orbs
    const createOrb = (color: string, baseRadius: number): Orb => ({
      x: Math.random() * canvas.offsetWidth,
      y: Math.random() * canvas.offsetHeight,
      baseRadius,
      radius: baseRadius,
      color,
      vx: (Math.random() - 0.5) * 0.4,
      vy: (Math.random() - 0.5) * 0.4,
      phase: Math.random() * Math.PI * 2,
      pulseSpeed: 0.006 + Math.random() * 0.006,
    });

    const orbs: Orb[] = [
      createOrb("rgba(139, 92, 246, 0.18)", 280),  // Royal violet - larger
      createOrb("rgba(192, 132, 252, 0.14)", 220), // Soft lavender
      createOrb("rgba(236, 72, 153, 0.12)", 200),  // Pink
      createOrb("rgba(212, 175, 55, 0.08)", 180),  // Champagne gold
      createOrb("rgba(99, 102, 241, 0.12)", 240),  // Indigo
      createOrb("rgba(244, 114, 182, 0.10)", 160), // Rose
    ];

    function updateOrb(orb: Orb) {
      orb.x += orb.vx;
      orb.y += orb.vy;
      orb.phase += orb.pulseSpeed;
      orb.radius = orb.baseRadius + Math.sin(orb.phase) * 30;

      const width = canvas.offsetWidth;
      const height = canvas.offsetHeight;
      if (orb.x < -orb.radius) orb.x = width + orb.radius;
      if (orb.x > width + orb.radius) orb.x = -orb.radius;
      if (orb.y < -orb.radius) orb.y = height + orb.radius;
      if (orb.y > height + orb.radius) orb.y = -orb.radius;
    }

    function drawOrb(orb: Orb) {
      if (!ctx) return;
      const gradient = ctx.createRadialGradient(
        orb.x,
        orb.y,
        0,
        orb.x,
        orb.y,
        orb.radius
      );
      gradient.addColorStop(0, orb.color);
      gradient.addColorStop(0.5, orb.color.replace(/[\d.]+\)$/, "0.05)"));
      gradient.addColorStop(1, "transparent");

      ctx.beginPath();
      ctx.arc(orb.x, orb.y, orb.radius, 0, Math.PI * 2);
      ctx.fillStyle = gradient;
      ctx.fill();
    }

    function animate() {
      if (!ctx) return;
      ctx.clearRect(0, 0, canvas.offsetWidth, canvas.offsetHeight);

      orbs.forEach((orb) => {
        updateOrb(orb);
        drawOrb(orb);
      });

      requestAnimationFrame(animate);
    }

    animate();

    return () => {
      window.removeEventListener("resize", setCanvasSize);
    };
  }, []);

  return <canvas ref={canvasRef} className={styles.floatingOrbs} />;
}

function HomepageHeader() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <header className={clsx("hero", styles.heroBanner)}>
      <FloatingOrbs />
      <div className="container">
        <h1 className={styles.heroTitle}>{siteConfig.title}</h1>
        <p className={styles.heroSubtitle}>
          A Kubernetes Query Language
        </p>
        <div className={styles.buttons}>
          <Link
            className={clsx(styles.button, styles.buttonPrimary)}
            to="/docs/installation"
          >
            Get Started
          </Link>
          <Link
            className={clsx(styles.button, styles.buttonSecondary)}
            href="https://github.com/avitaltamir/cyphernetes"
          >
            View on GitHub
          </Link>
        </div>
        <div className={styles.heroImage}>
          <img
            src="/img/visualization.png"
            alt="Cyphernetes Visualization"
            width="100%"
            style={{ maxWidth: "800px", marginTop: "2rem" }}
          />
        </div>
      </div>
    </header>
  );
}

const beforeExample = `# Delete all non-running pods
$ kubectl get pods --all-namespaces \\
  --field-selector 'status.phase!=Running' \\
  -o 'custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name' \\
  --no-headers | xargs -L1 -I {} bash -c 'set -- {}; kubectl delete pod $2 -n $1'`;

const afterExample = `// Do the same thing!
MATCH (p:Pod)
WHERE p.status.phase != "Running"
DELETE p;`;

function CodeComparison() {
  return (
    <section className={styles.codeComparison}>
      <div className="container">
        <div className="row">
          <div className="col col--6">
            <div className={styles.codeBlockTitle}>
              <span className={styles.labelBadge} data-type="before">
                Complex
              </span>
              <span>Traditional kubectl</span>
            </div>
            <div className={styles.codeBlock}>
              <CodeBlock language="bash" showLineNumbers>
                {beforeExample}
              </CodeBlock>
            </div>
          </div>
          <div className="col col--6">
            <div className={styles.codeBlockTitle}>
              <span className={styles.labelBadge} data-type="after">
                Simple
              </span>
              <span>With Cyphernetes</span>
            </div>
            <div className={styles.codeBlock}>
              <CodeBlock language="graphql" showLineNumbers>
                {afterExample}
              </CodeBlock>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

function Feature({
  title,
  description,
  emoji,
}: {
  title: string;
  description: ReactNode;
  emoji: string;
}) {
  return (
    <div className={styles.featureCard}>
      <div className={styles.emoji}>{emoji}</div>
      <h3 className={styles.featureTitle}>{title}</h3>
      <p className={styles.featureDescription}>{description}</p>
    </div>
  );
}

function HomepageFeatures() {
  return (
    <section className={styles.features}>
      <div className="container">
        <h2 className={styles.sectionTitle}>Why Cyphernetes?</h2>
        <div className={styles.grid}>
          <Feature
            emoji="✨"
            title="Graph-Powered Queries"
            description="Express complex multi-resource operations with elegant Cypher syntax. Relationships between resources become first-class citizens."
          />
          <Feature
            emoji="⚡"
            title="Zero Configuration"
            description="Works instantly with any cluster. Automatically discovers all your CRDs and understands resource relationships out of the box."
          />
          <Feature
            emoji="🌐"
            title="Multi-Cluster Native"
            description="Query across your entire infrastructure with a single command. One syntax to rule all your Kubernetes environments."
          />
        </div>
      </div>
    </section>
  );
}

function GrowingEcosystem() {
  return (
    <section className={clsx(styles.section, styles.sectionAlt)}>
      <div className="container">
        <h2 className={styles.sectionTitle}>One Language, Many Interfaces</h2>
        <div className={styles.ecosystemGrid}>
          <div className={styles.ecosystemCard}>
            <div className={styles.ecosystemIcon}>$_</div>
            <h3>Interactive Shell</h3>
            <p>
              Powerful REPL with intelligent auto-completion, syntax highlighting,
              and built-in macros for common operations.
            </p>
          </div>
          <div className={styles.ecosystemCard}>
            <div className={styles.ecosystemIcon}>&lt;/&gt;</div>
            <h3>Web Interface</h3>
            <p>
              Visualize your cluster as a graph. See relationships between
              resources and execute queries with instant visual feedback.
            </p>
          </div>
          <div className={styles.ecosystemCard}>
            <div className={styles.ecosystemIcon}>&infin;</div>
            <h3>Dynamic Operators</h3>
            <p>
              Define reactive workflows with Cyphernetes queries. Build
              production-ready operators without writing Go code.
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  const { siteConfig } = useDocusaurusContext();
  return (
    <Layout
      title={siteConfig.title}
      description="Query Kubernetes like a graph database. Cyphernetes brings Cypher-inspired syntax to kubectl, making complex operations simple and intuitive."
    >
      <HomepageHeader />
      <main>
        <CodeComparison />
        <HomepageFeatures />
        <GrowingEcosystem />
      </main>
    </Layout>
  );
}
