import { type ReactNode, useEffect, useRef } from "react";
import clsx from "clsx";
import Link from "@docusaurus/Link";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";
import Layout from "@theme/Layout";
import CodeBlock from "@theme/CodeBlock";

import styles from "./index.module.css";

function GraphParticles() {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    // Set canvas size
    const setCanvasSize = () => {
      canvas.width = canvas.offsetWidth;
      canvas.height = canvas.offsetHeight;
    };
    setCanvasSize();
    window.addEventListener("resize", setCanvasSize);

    // Particle class
    class Particle {
      x: number;
      y: number;
      vx: number;
      vy: number;
      size: number;

      constructor() {
        this.x = Math.random() * canvas.width;
        this.y = Math.random() * canvas.height;
        this.vx = (Math.random() - 0.5) * 0.3;
        this.vy = (Math.random() - 0.5) * 0.3;
        this.size = Math.random() * 3 + 2;
      }

      update() {
        this.x += this.vx;
        this.y += this.vy;

        if (this.x < 0) this.x = canvas.width;
        if (this.x > canvas.width) this.x = 0;
        if (this.y < 0) this.y = canvas.height;
        if (this.y > canvas.height) this.y = 0;
      }

      draw() {
        if (!ctx) return;
        ctx.beginPath();
        ctx.arc(this.x, this.y, this.size, 0, Math.PI * 2);
        ctx.fillStyle = "rgba(255, 255, 255, 0.5)";
        ctx.fill();
      }
    }

    // Create particles
    const particles: Particle[] = [];
    for (let i = 0; i < 70; i++) {
      particles.push(new Particle());
    }

    // Animation loop
    function animate() {
      if (!ctx) return;
      ctx.clearRect(0, 0, canvas.width, canvas.height);

      // Update and draw particles
      particles.forEach((particle) => {
        particle.update();
        particle.draw();
      });

      // Draw connections
      particles.forEach((p1, i) => {
        particles.slice(i + 1).forEach((p2) => {
          const dx = p1.x - p2.x;
          const dy = p1.y - p2.y;
          const distance = Math.sqrt(dx * dx + dy * dy);

          if (distance < 150) {
            ctx.beginPath();
            ctx.moveTo(p1.x, p1.y);
            ctx.lineTo(p2.x, p2.y);
            ctx.strokeStyle = `rgba(255, 255, 255, ${
              0.3 * (1 - distance / 150)
            })`;
            ctx.lineWidth = 1;
            ctx.stroke();
          }
        });
      });

      requestAnimationFrame(animate);
    }

    animate();

    return () => {
      window.removeEventListener("resize", setCanvasSize);
    };
  }, []);

  return <canvas ref={canvasRef} className={styles.particles} />;
}

function HomepageHeader() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <header className={clsx("hero", styles.heroBanner)}>
      <GraphParticles />
      <div className="container">
        <h1 className={styles.heroTitle}>{siteConfig.title}</h1>
        <p className={styles.heroSubtitle}>{siteConfig.tagline}</p>
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
            src="https://cyphernet.es/media/dfc9cea8094d9c8e54bad630359ab252.png"
            alt="Cyphernetes Visualization"
            width="100%"
            style={{ maxWidth: "800px", marginTop: "2rem" }}
          />
        </div>
      </div>
    </header>
  );
}

const beforeExample = `# Delete all pods that are not running
$ kubectl get pods --all-namespaces \\
    --field-selector 'status.phase!=Running' \\
    -o 'custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name' \\
    --no-headers | xargs -L1 -I {} bash -c 'set -- {}; kubectl delete pod $2 -n $1'`;

const afterExample = `# Do the same thing!
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
              <span className={styles.emoji}>😣</span>
              <span>Before Cyphernetes</span>
            </div>
            <div className={styles.codeBlock}>
              <CodeBlock language="bash" showLineNumbers>
                {beforeExample}
              </CodeBlock>
            </div>
          </div>
          <div className="col col--6">
            <div className={styles.codeBlockTitle}>
              <span className={styles.emoji}>🤩</span>
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
            emoji="🎯"
            title="Intuitive Graph Queries"
            description="Use Cypher-inspired syntax to query and manipulate your Kubernetes resources naturally, just like you would with a graph database."
          />
          <Feature
            emoji="🚀"
            title="Works Out of the Box"
            description="No setup required. Cyphernetes works with your existing Kubernetes clusters and automatically supports all your CRDs."
          />
          <Feature
            emoji="🌐"
            title="Multi-Cluster Support"
            description="Query and manage resources across multiple clusters with the same simple syntax. Perfect for complex, distributed environments."
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
        <h2 className={styles.sectionTitle}>A Growing Ecosystem</h2>
        <div className={styles.ecosystemGrid}>
          <div className={styles.ecosystemCard}>
            <img
              src="https://cyphernet.es/media/72f1a5fe67e738dd69972bc0ec7d4acf.png"
              alt="Interactive Shell"
              className={styles.ecosystemImage}
            />
            <h3>Fully-Featured Interactive Shell</h3>
            <p>
              Powerful interactive shell with auto-completion and syntax
              highlighting.
            </p>
          </div>
          <div className={styles.ecosystemCard}>
            <img
              src="https://cyphernet.es/media/44867f192636ac9cde31e5f91f64d620.png"
              alt="Web Client"
              className={styles.ecosystemImage}
            />
            <h3>Beautiful Web Client</h3>
            <p>Experience Kubernetes in a whole new way with the Web UI.</p>
          </div>
          <div className={styles.ecosystemCard}>
            <img
              src="https://cyphernet.es/media/d4edb7f7277955da813f0565055c7989.png"
              alt="K8s Operators"
              className={styles.ecosystemImage}
            />
            <h3>Instant K8s Operators</h3>
            <p>Plug it into CI/CD. Spin up Operators in minutes.</p>
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
      description="A Cypher-inspired Kubernetes query language"
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
