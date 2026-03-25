import commitOptionsSchema from "../schemas/commit-options.json";

export interface CommitOption {
	subject: string;
	body?: string;
}

export interface CommitOptionsOutput {
	options: CommitOption[];
}

export { commitOptionsSchema };
