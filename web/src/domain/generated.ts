// Wire types for thingd's JSON API. Hand-maintained (this project does not use
// codegen) — keep in sync with the Go side: internal/exporter (tree/Node) and
// internal/model (Link, enums).

/**
 * A node in the exported tree (Epic > Issue > Task).
 */
export interface Node {
    body?:     string;
    category?: string;
    children?: Node[];
    links?:    NodeLink[];
    priority?: Priority;
    /**
     * The node's full slug-path identity (e.g. "epic/issue/task"), used to
     * address it in the API. A slug is unique only among siblings; the ref is
     * unique across the tree.
     */
    ref:       string;
    slug:      string;
    status:    Status;
    tags?:     string[];
    title:     string;
    type:      Type;
    updated?:  string;
}

export interface NodeLink {
    label?: string;
    url:    string;
}

export enum Priority {
    High = "high",
    Low = "low",
    Medium = "medium",
}

export enum Status {
    Doing = "doing",
    Done = "done",
    Paused = "paused",
    Todo = "todo",
}

export enum Type {
    Epic = "epic",
    Issue = "issue",
    Task = "task",
}
