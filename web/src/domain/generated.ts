// Code generated from schema/tree.json by quicktype; DO NOT EDIT.
// Regenerate with `make gen` after changing the schema.

/**
 * A node in the exported tree (Epic > Issue > Task).
 */
export interface Node {
    body?:     string;
    category?: string;
    children?: Node[];
    /**
     * The derived display status (own status, or the rollup from children). Use this to show
     * and filter a node's status.
     */
    effectiveStatus: Status;
    links?:          NodeLink[];
    priority?:       Priority;
    /**
     * The node's full slug-path identity (e.g. "epic/issue/task"), used to address it in the
     * API. Its bare slug is the last segment; the ref is unique across the tree, a slug only
     * among siblings.
     */
    ref: string;
    /**
     * The node's own stored status, present only when it has one. Absent means the status is
     * rolled up from children (see effectiveStatus); a parent can be reverted to the rollup by
     * setting an empty status.
     */
    status?:  Status;
    tags?:    string[];
    title:    string;
    type:     Type;
    updated?: string;
}

/**
 * The derived display status (own status, or the rollup from children). Use this to show
 * and filter a node's status.
 *
 * The node's own stored status, present only when it has one. Absent means the status is
 * rolled up from children (see effectiveStatus); a parent can be reverted to the rollup by
 * setting an empty status.
 */
export enum Status {
    Doing = "doing",
    Done = "done",
    Paused = "paused",
    Todo = "todo",
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

export enum Type {
    Epic = "epic",
    Issue = "issue",
    Task = "task",
}
