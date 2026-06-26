import React from "react";
import ReactDOM from "react-dom/client";
import { Github, Menu, Search, X } from "lucide-react";
import { docs, sections, type DocPage } from "./generated/docs";
import "./styles.css";

const docByRoute = new Map(docs.map((doc) => [doc.route, doc]));
const defaultRoute = docs[0]?.route ?? "start/overview";
const mascotSrc = "gefahr_turtle_v3.png";

function App() {
  const [route, setRoute] = React.useState(currentRoute);
  const [query, setQuery] = React.useState("");
  const [mobileNavOpen, setMobileNavOpen] = React.useState(false);
  const activeDoc = docByRoute.get(route) ?? docByRoute.get(defaultRoute) ?? docs[0];
  const filteredDocs = useSearch(query);

  React.useEffect(() => {
    const onHashChange = () => setRoute(currentRoute());
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  React.useEffect(() => {
    document.title = activeDoc ? `${activeDoc.title} | Gefahr Documentation` : "Gefahr Documentation";
    setMobileNavOpen(false);
  }, [activeDoc?.route]);

  return (
    <div className="app-shell">
      <header className="topbar">
        <button className="icon-button mobile-only" type="button" onClick={() => setMobileNavOpen(true)} aria-label="Open navigation">
          <Menu size={19} />
        </button>
        <a className="brand" href={`#/${defaultRoute}`} aria-label="Gefahr documentation home">
          <img className="brand-mascot" src={mascotSrc} alt="" aria-hidden="true" />
          <span>
            <strong>Gefahr</strong>
            <small>Docs</small>
          </span>
        </a>
        <div className="toolbar">
          <select aria-label="Documentation version" defaultValue="main">
            <option value="main">main</option>
            <option value="v1">v1.x</option>
          </select>
          <a className="icon-button" href="https://github.com/ANRorg/Gefahr" aria-label="Open GitHub repository">
            <Github size={18} />
          </a>
        </div>
      </header>

      <div className="content-shell">
        <aside className={`sidebar ${mobileNavOpen ? "open" : ""}`}>
          <div className="sidebar-header mobile-only">
            <strong>Documentation</strong>
            <button className="icon-button" type="button" onClick={() => setMobileNavOpen(false)} aria-label="Close navigation">
              <X size={18} />
            </button>
          </div>
          <div className="sidebar-search">
            <Search size={18} aria-hidden="true" />
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search this site" />
          </div>
          <Navigation activeRoute={activeDoc?.route ?? defaultRoute} />
        </aside>

        <main className="main-pane">
          {query.trim() ? <SearchResults query={query} results={filteredDocs} /> : activeDoc ? <DocView doc={activeDoc} /> : null}
        </main>
      </div>
    </div>
  );
}

function Navigation({ activeRoute }: { activeRoute: string }) {
  const [openSections, setOpenSections] = React.useState<Set<string>>(() => {
    const initial = new Set<string>(["Start", "Concepts"]);
    const activeSection = sections.find((section) => section.docs.includes(activeRoute));
    if (activeSection) initial.add(activeSection.title);
    return initial;
  });

  React.useEffect(() => {
    const activeSection = sections.find((section) => section.docs.includes(activeRoute));
    if (!activeSection) return;
    setOpenSections((current) => new Set(current).add(activeSection.title));
  }, [activeRoute]);

  function toggleSection(title: string) {
    setOpenSections((current) => {
      const next = new Set(current);
      if (next.has(title)) {
        next.delete(title);
      } else {
        next.add(title);
      }
      return next;
    });
  }

  return (
    <nav className="nav-tree" aria-label="Documentation navigation">
      <h2>Documentation</h2>
      {sections.map((section) => (
        <section key={section.title} className="nav-section">
          <button
            className="nav-section-toggle"
            type="button"
            aria-expanded={openSections.has(section.title)}
            onClick={() => toggleSection(section.title)}
          >
            <span className="disclosure" aria-hidden="true" />
            {section.title}
          </button>
          {openSections.has(section.title) ? (
            <div className="nav-section-links">
              {section.docs.map((route) => {
                const doc = docByRoute.get(route);
                if (!doc) return null;
                return (
                  <a key={route} className={activeRoute === route ? "active" : ""} href={`#/${route}`}>
                    {doc.title}
                  </a>
                );
              })}
            </div>
          ) : null}
        </section>
      ))}
    </nav>
  );
}

function DocView({ doc }: { doc: DocPage }) {
  const articleRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    enhanceArticle();
    const pendingAnchor = sessionStorage.getItem("gefahr-docs-anchor");
    if (pendingAnchor) {
      sessionStorage.removeItem("gefahr-docs-anchor");
      window.setTimeout(() => scrollToAnchor(pendingAnchor), 0);
    } else {
      window.scrollTo({ top: 0, behavior: "instant" });
    }
  }, [doc.route]);

  function onArticleClick(event: React.MouseEvent<HTMLElement>) {
    const link = (event.target as Element).closest<HTMLAnchorElement>("a[data-doc-anchor]");
    if (!link) return;
    const anchor = link.dataset.docAnchor;
    const targetRoute = link.getAttribute("href")?.replace(/^#\/?/, "");
    if (!anchor || !targetRoute) return;
    event.preventDefault();
    if (targetRoute !== doc.route) {
      sessionStorage.setItem("gefahr-docs-anchor", anchor);
      window.location.hash = `/${targetRoute}`;
      return;
    }
    scrollToAnchor(anchor);
  }

  return (
    <article className="doc-layout">
      <div className="doc-main">
        <header className="article-heading">
          <div className="eyebrow">
            <span>{doc.section}</span>
            <span>{doc.readingMinutes} min read</span>
          </div>
          <h1>{doc.title}</h1>
          <p>{doc.summary}</p>
          <a className="source-link" href={`https://github.com/ANRorg/Gefahr/blob/main/${doc.sourcePath}`}>
            Edit this page: {doc.sourcePath}
          </a>
        </header>
        <div ref={articleRef} className="article-body" onClick={onArticleClick} dangerouslySetInnerHTML={{ __html: doc.html }} />
      </div>
      <aside className="toc" aria-label="On this page">
        <strong>On this page</strong>
        {doc.headings.length ? (
          doc.headings.map((heading) => (
            <button key={heading} type="button" onClick={() => scrollToAnchor(slugify(heading))}>
              {heading}
            </button>
          ))
        ) : (
          <span>No sections</span>
        )}
      </aside>
    </article>
  );
}

function SearchResults({ query, results }: { query: string; results: DocPage[] }) {
  return (
    <section className="search-results">
      <header>
        <div className="eyebrow">
          <Search size={16} />
          Search
        </div>
        <h1>{results.length} results for "{query}"</h1>
        <p>Results include guides, concepts, tasks, operations pages, and reference material.</p>
      </header>
      <div className="result-list">
        {results.map((doc) => (
          <a key={doc.route} href={`#/${doc.route}`}>
            <span className="result-section">{doc.section}</span>
            <strong>{doc.title}</strong>
            <p>{excerpt(doc.text, query)}</p>
          </a>
        ))}
      </div>
    </section>
  );
}

function useSearch(query: string) {
  return React.useMemo(() => {
    const normalized = query.trim().toLowerCase();
    if (!normalized) return docs;
    return docs
      .map((doc) => {
        const haystack = `${doc.title} ${doc.section} ${doc.summary} ${doc.headings.join(" ")} ${doc.text}`.toLowerCase();
        const titleScore = doc.title.toLowerCase().includes(normalized) ? 12 : 0;
        const headingScore = doc.headings.join(" ").toLowerCase().includes(normalized) ? 6 : 0;
        const bodyScore = haystack.includes(normalized) ? 1 : 0;
        return { doc, score: titleScore + headingScore + bodyScore };
      })
      .filter((entry) => entry.score > 0)
      .sort((a, b) => b.score - a.score || a.doc.sectionOrder - b.doc.sectionOrder)
      .map((entry) => entry.doc);
  }, [query]);
}

function currentRoute() {
  return window.location.hash.replace(/^#\/?/, "") || defaultRoute;
}

function enhanceArticle() {
  document.querySelectorAll<HTMLPreElement>(".article-body pre").forEach((pre) => {
    if (pre.querySelector("button")) return;
    const button = document.createElement("button");
    button.type = "button";
    button.className = "copy-button";
    button.innerHTML = `<span>Copy</span>`;
    button.addEventListener("click", async () => {
      await navigator.clipboard.writeText(pre.innerText);
      button.innerHTML = `<span>Copied</span>`;
      window.setTimeout(() => {
        button.innerHTML = `<span>Copy</span>`;
      }, 1400);
    });
    pre.appendChild(button);
  });
}

function scrollToAnchor(anchor: string) {
  const decoded = decodeURIComponent(anchor);
  const target = document.getElementById(decoded) ?? document.getElementById(slugify(decoded));
  target?.scrollIntoView({ behavior: "smooth", block: "start" });
}

function slugify(value: string) {
  return value
    .toLowerCase()
    .replace(/[^\w\s-]/g, "")
    .trim()
    .replace(/\s+/g, "-");
}

function excerpt(text: string, query: string) {
  const normalized = query.trim().toLowerCase();
  const index = text.toLowerCase().indexOf(normalized);
  if (index < 0) return text.slice(0, 180);
  const start = Math.max(0, index - 70);
  return `${start > 0 ? "..." : ""}${text.slice(start, start + 220)}${start + 220 < text.length ? "..." : ""}`;
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
