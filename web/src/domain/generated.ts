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
    /**
     * The derived display status (own status, or the rollup from children). Use
     * this to show and filter a node's status.
     */
    effectiveStatus: Status;
    links?:    NodeLink[];
    priority?: Priority;
    /**
     * The node's full slug-path identity (e.g. "epic/issue/task"), used to
     * address it in the API. Its bare slug is the last segment; the ref is
     * unique across the tree, a slug only among siblings.
     */
    ref:       string;
    /**
     * The node's own stored status, present only when it has one. Absent means
     * the status is rolled up from children (see effectiveStatus); a parent can be
     * reverted to the rollup by setting an empty status.
     */
    status?:   Status;
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
