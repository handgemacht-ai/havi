import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";

const CHANNEL_PORT = parseInt(process.env["CHANNEL_PORT"] || "8091", 10);
const ANNOTATION_SERVER_URL =
  process.env["ANNOTATION_SERVER_URL"] || "http://localhost:8090";

interface AnnotationWebhookPayload {
  id: string;
  annotation: {
    "@context": string;
    id: string;
    type: string;
    motivation?: string;
    created?: string;
    modified?: string;
    creator?: { type: string; name: string };
    body: Array<{ type: string; value?: string; purpose?: string; id?: string }>;
    target: {
      source: string;
      selector?: Array<{
        type: string;
        value: string;
        conformsTo?: string;
      }>;
      state?: { type: string; value: string };
    };
  };
  domain: string;
  worktree: string;
  branch: string;
  state: string;
  motivation: string;
  created_at: string;
  updated_at: string;
}

const INSTRUCTIONS = `You receive notifications as <channel source="annotations-channel"> events when developers capture annotations — visual observations of UI issues from the browser. Each notification includes the annotation ID, comment text, page URL, and project context (domain, worktree, branch) as tag attributes.

Annotations may arrive one at a time (automatic mode) or in a burst (deferred mode, when the developer clicks "Send to Claude Code"). When you receive multiple annotations at once, read all of them first, then prioritize and work through them.

To act on an annotation:
1. Read the comment and context from the notification
2. Use the list_annotations MCP tool to get full annotation details if needed
3. Use the get_annotation_image MCP tool to view the screenshot
4. Locate the relevant source code in the current worktree
5. If the annotation describes a code issue you can fix, make the fix
6. Call the resolve_annotation tool (on this channel) with the annotation ID, commit hash, and description of what you changed

If you cannot fix the issue (wrong worktree, unclear description, infrastructure problem), explain why and suggest next steps.`;

const mcpServer = new McpServer(
  { name: "annotations", version: "1.0.0" },
  {
    capabilities: {
      experimental: { "claude/channel": {} },
      tools: {},
    },
    instructions: INSTRUCTIONS,
  }
);

mcpServer.registerTool(
  "resolve_annotation",
  {
    description:
      "Mark an annotation as resolved after fixing the issue it describes",
    inputSchema: z.object({
      annotation_id: z.string().describe("UUID of the annotation to resolve"),
      commit: z.string().optional().describe("Commit hash that contains the fix"),
      pr: z.string().optional().describe("PR number or URL"),
      description: z
        .string()
        .optional()
        .describe("Description of what was changed"),
    }),
  },
  async ({ annotation_id, commit, pr, description }) => {
    const url = `${ANNOTATION_SERVER_URL}/api/annotations/${annotation_id}/resolve`;
    try {
      const res = await fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          resolution: {
            ...(commit && { commit }),
            ...(pr && { pr }),
            ...(description && { description }),
          },
        }),
      });

      if (res.status === 200) {
        return {
          content: [
            { type: "text" as const, text: `Annotation ${annotation_id} resolved.` },
          ],
        };
      }
      if (res.status === 404) {
        return {
          content: [
            { type: "text" as const, text: `Annotation ${annotation_id} not found.` },
          ],
          isError: true,
        };
      }
      if (res.status === 409) {
        return {
          content: [
            {
              type: "text" as const,
              text: `Annotation ${annotation_id} is already resolved.`,
            },
          ],
          isError: true,
        };
      }

      const body = await res.text();
      return {
        content: [
          {
            type: "text" as const,
            text: `Unexpected response (${res.status}): ${body}`,
          },
        ],
        isError: true,
      };
    } catch (err) {
      return {
        content: [
          {
            type: "text" as const,
            text: `Failed to reach annotation server: ${err instanceof Error ? err.message : String(err)}`,
          },
        ],
        isError: true,
      };
    }
  }
);

function extractComment(payload: AnnotationWebhookPayload): string {
  const textBody = payload.annotation.body.find(
    (b) => b.type === "TextualBody" && b.purpose === "commenting"
  );
  return textBody?.value ?? "(no comment)";
}

async function handleWebhook(req: Request): Promise<Response> {
  let payload: AnnotationWebhookPayload;
  try {
    payload = (await req.json()) as AnnotationWebhookPayload;
  } catch {
    return Response.json(
      { error: { code: "bad_request", message: "Invalid JSON" } },
      { status: 400 }
    );
  }

  if (!payload.id || !payload.annotation?.target?.source) {
    return Response.json(
      { error: { code: "bad_request", message: "Missing required fields" } },
      { status: 400 }
    );
  }

  const comment = extractComment(payload);
  const pageUrl = payload.annotation.target.source;

  const notification = {
    method: "notifications/claude/channel",
    params: {
      content: `New annotation: "${comment}"\nPage: ${pageUrl}\nID: ${payload.id}`,
      meta: {
        annotation_id: payload.id,
        domain: payload.domain || "",
        worktree: payload.worktree || "",
        branch: payload.branch || "",
        page_url: pageUrl,
      },
    },
  };

  try {
    await mcpServer.server.notification(notification as any);
  } catch (err) {
    console.error("Channel notification failed:", err);
  }

  return Response.json({ status: "accepted" });
}

const transport = new StdioServerTransport();
await mcpServer.connect(transport);
console.error(`MCP server connected on stdio`);

Bun.serve({
  port: CHANNEL_PORT,
  async fetch(req) {
    const url = new URL(req.url);

    if (req.method === "POST" && url.pathname === "/webhook/annotation") {
      return handleWebhook(req);
    }

    if (req.method === "GET" && url.pathname === "/health") {
      return Response.json({ status: "ok" });
    }

    return Response.json(
      { error: { code: "not_found", message: "Not found" } },
      { status: 404 }
    );
  },
});

console.error(`HTTP listener on port ${CHANNEL_PORT}`);
