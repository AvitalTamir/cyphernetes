import type { ReactNode } from "react";
import clsx from "clsx";
import Link from "@docusaurus/Link";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";
import Layout from "@theme/Layout";
import CodeBlock from "@theme/CodeBlock";

import styles from "./index.module.css";

function HomepageHeader() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <header className={clsx("hero", styles.heroBanner)}>
      <div className="container">
        <h1 className={styles.heroTitle}>{siteConfig.title}</h1>
        <p className={styles.heroSubtitle}>{siteConfig.tagline}</p>
        <div className={styles.buttons}>
          <Link
            className={clsx(styles.button, styles.buttonPrimary)}
            to="/docs/intro"
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
      </main>
    </Layout>
  );
}
