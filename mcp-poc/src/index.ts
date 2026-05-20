#!/usr/bin/env node

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { z } from "zod";
import { exec, spawn } from "child_process";
import { promisify } from "util";
import express, { Request, Response } from "express";

const execAsync = promisify(exec);

// Track running operations
const runningOperations = new Map<string, {
  command: string;
  startTime: Date;
  status: 'running' | 'completed' | 'failed';
  output: string[];
  error?: string;
}>();

// Create server instance
const server = new McpServer(
  {
    name: "mcp-poc",
    version: "1.0.0",
  },
  {
    capabilities: {
      tools: {},
    },
  }
);

// Define the input schema using Zod
const CreateApplicationSchema = z.object({
  name: z.string().describe("Application name (required)"),
  template: z.string().describe("Application template to use (required). Use 'ai-services application templates' to view available templates."),
  params: z.array(z.string()).optional().describe("Inline parameters to configure the application. Format: key=value pairs. Example: ['key1=value1', 'key2=value2']. These override values from --values files."),
  values: z.array(z.string()).optional().describe("Specify values files to override default template values. Can provide multiple files; later files override earlier ones."),
  skipValidation: z.array(z.string()).optional().describe("Skip specific validation checks during application creation."),
  skipImageDownload: z.boolean().optional().describe("Skip container image pull/download during application creation. Use only if required images already exist locally. (Podman runtime only)"),
  skipModelDownload: z.boolean().optional().describe("Skip model download during application creation. Use if local models already exist at /var/lib/ai-services/models/. (Podman runtime only)"),
  imagePullPolicy: z.enum(["Always", "Never", "IfNotPresent"]).optional().describe("Image pull policy for container images. 'Always': pull every time, 'Never': use only local images, 'IfNotPresent': pull only if not present locally. Defaults to 'IfNotPresent'. (Podman runtime only)"),
  timeout: z.string().optional().describe("Timeout for the operation (e.g., '10s', '2m', '1h'). (OpenShift runtime only)"),
});

// Register the tool using the modern API
server.registerTool(
  "create_application",
  {
    description: "Creates an AI services application using the ai-services CLI. Deploys an application with the provided name based on the specified template.",
    inputSchema: CreateApplicationSchema,
  },
  async (args) => {
    const typedArgs = args as z.infer<typeof CreateApplicationSchema>;

    // Validate required arguments
    if (!typedArgs.name) {
      throw new Error("Missing required argument: name");
    }
    if (!typedArgs.template) {
      throw new Error("Missing required argument: template");
    }

    // Build the command
    let command = `ai-services application create ${typedArgs.name} --template ${typedArgs.template} --runtime podman`;

    // Add optional flags
    if (typedArgs.params && typedArgs.params.length > 0) {
      command += ` --params ${typedArgs.params.join(",")}`;
    }

    if (typedArgs.values && typedArgs.values.length > 0) {
      for (const valueFile of typedArgs.values) {
        command += ` --values ${valueFile}`;
      }
    }

    if (typedArgs.skipValidation && typedArgs.skipValidation.length > 0) {
      command += ` --skip-validation ${typedArgs.skipValidation.join(",")}`;
    }

    if (typedArgs.skipImageDownload === true) {
      command += " --skip-image-download";
    }

    if (typedArgs.skipModelDownload === true) {
      command += " --skip-model-download";
    }

    if (typedArgs.imagePullPolicy) {
      command += ` --image-pull-policy ${typedArgs.imagePullPolicy}`;
    }

    if (typedArgs.timeout) {
      command += ` --timeout ${typedArgs.timeout}`;
    }

    // Generate operation ID
    const operationId = `${typedArgs.name}-${Date.now()}`;

    // Initialize operation tracking
    runningOperations.set(operationId, {
      command,
      startTime: new Date(),
      status: 'running',
      output: []
    });

    // Execute command asynchronously without waiting
    const child = spawn('sh', ['-c', command], {
      stdio: ['ignore', 'pipe', 'pipe']
    });

    let stdout = '';
    let stderr = '';

    child.stdout?.on('data', (data) => {
      const text = data.toString();
      stdout += text;
      const op = runningOperations.get(operationId);
      if (op) {
        op.output.push(text);
      }
    });

    child.stderr?.on('data', (data) => {
      const text = data.toString();
      stderr += text;
      const op = runningOperations.get(operationId);
      if (op) {
        op.output.push(text);
      }
    });

    child.on('close', (code) => {
      const op = runningOperations.get(operationId);
      if (op) {
        op.status = code === 0 ? 'completed' : 'failed';
        if (code !== 0) {
          op.error = `Process exited with code ${code}`;
        }
      }
    });

    // Return immediately with operation ID
    return {
      content: [
        {
          type: "text",
          text: JSON.stringify(
            {
              success: true,
              operationId,
              message: "Application creation started. Use check_operation_status to monitor progress.",
              command,
              note: "This is a long-running operation. The process is running in the background."
            },
            null,
            2
          ),
        },
      ],
    };
  }
);

// Register status checking tool
const CheckOperationStatusSchema = z.object({
  operationId: z.string().describe("The operation ID returned from create_application"),
});

server.registerTool(
  "check_operation_status",
  {
    description: "Check the status of a long-running application creation operation. Returns current status, output, and completion state.",
    inputSchema: CheckOperationStatusSchema,
  },
  async (args) => {
    const typedArgs = args as z.infer<typeof CheckOperationStatusSchema>;

    const operation = runningOperations.get(typedArgs.operationId);

    if (!operation) {
      return {
        content: [
          {
            type: "text",
            text: JSON.stringify(
              {
                success: false,
                error: "Operation not found",
                operationId: typedArgs.operationId
              },
              null,
              2
            ),
          },
        ],
        isError: true,
      };
    }

    const duration = Date.now() - operation.startTime.getTime();
    const durationSeconds = Math.floor(duration / 1000);

    return {
      content: [
        {
          type: "text",
          text: JSON.stringify(
            {
              success: true,
              operationId: typedArgs.operationId,
              status: operation.status,
              command: operation.command,
              duration: `${durationSeconds}s`,
              output: operation.output.slice(-20), // Last 20 lines
              error: operation.error,
              isComplete: operation.status !== 'running'
            },
            null,
            2
          ),
        },
      ],
    };
  }
);

// Register list operations tool
server.registerTool(
  "list_operations",
  {
    description: "List all tracked operations (running, completed, and failed)",
    inputSchema: z.object({}),
  },
  async () => {
    const operations = Array.from(runningOperations.entries()).map(([id, op]) => ({
      operationId: id,
      status: op.status,
      command: op.command,
      startTime: op.startTime.toISOString(),
      duration: `${Math.floor((Date.now() - op.startTime.getTime()) / 1000)}s`
    }));

    return {
      content: [
        {
          type: "text",
          text: JSON.stringify(
            {
              success: true,
              operations,
              count: operations.length
            },
            null,
            2
          ),
        },
      ],
    };
  }
);

// Start the server with StreamableHTTP transport over HTTP
async function main() {
  const app = express();
  const PORT = parseInt(process.env.PORT || "3000", 10);
  const HOST = process.env.HOST || "0.0.0.0";

  // Parse JSON bodies before handling requests
  app.use(express.json());

  app.get("/health", (_req: Request, res: Response) => {
    res.json({ status: "ok", server: "mcp-poc" });
  });

  // Handle MCP requests using the StreamableHTTP transport
  app.post("/mcp", async (req: Request, res: Response) => {
    console.error("New MCP request received");
    console.error("Request body:", JSON.stringify(req.body, null, 2));

    try {
      // Create a new transport instance for each request
      const transport = new StreamableHTTPServerTransport();

      // Close any existing connection before connecting a new one
      try {
        await server.server.close();
      } catch (closeError) {
        // Ignore errors if not connected
        console.error("Note: No existing connection to close");
      }

      // Connect the server to the new transport
      await server.server.connect(transport);

      // Handle the request
      await transport.handleRequest(req, res, req.body);

      // Close the connection after handling the request
      await server.server.close();
    } catch (error: any) {
      console.error("Error handling MCP request:", error);
      if (!res.headersSent) {
        res.status(500).json({
          error: "Internal server error",
          message: error.message,
          stack: error.stack
        });
      }
    }
  });

  app.listen(PORT, HOST as string, () => {
    console.error(`MCP POC Server running on http://${HOST}:${PORT}`);
    console.error(`MCP endpoint: http://${HOST}:${PORT}/mcp`);
    console.error(`Health check: http://${HOST}:${PORT}/health`);
  });
}

main().catch((error) => {
  console.error("Fatal error in main():", error);
  process.exit(1);
});