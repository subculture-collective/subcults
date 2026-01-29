/**
 * Mock API Server for E2E Testing
 * 
 * Simulates the Subcults API server endpoints needed for streaming tests.
 * Primarily handles LiveKit token generation.
 */

import express, { Express, Request, Response } from 'express';
import type { Server } from 'http';
import { MockLiveKitServer } from './mock-livekit-server';

export class MockAPIServer {
  private app: Express;
  private server: Server | null = null;
  private mockLiveKit: MockLiveKitServer;
  
  constructor(
    private port: number = 8080,
    private liveKitPort: number = 7880
  ) {
    this.app = express();
    this.mockLiveKit = new MockLiveKitServer(liveKitPort);
    this.setupMiddleware();
    this.setupRoutes();
  }

  /**
   * Setup Express middleware
   */
  private setupMiddleware(): void {
    this.app.use(express.json());
    
    // CORS for testing
    this.app.use((req, res, next) => {
      res.header('Access-Control-Allow-Origin', '*');
      res.header('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
      res.header('Access-Control-Allow-Headers', 'Content-Type, Authorization');
      
      if (req.method === 'OPTIONS') {
        res.sendStatus(200);
      } else {
        next();
      }
    });
  }

  /**
   * Setup API routes
   */
  private setupRoutes(): void {
    // Health check
    this.app.get('/health', (req: Request, res: Response) => {
      res.json({ status: 'ok', timestamp: new Date().toISOString() });
    });

    // LiveKit token generation (matches production API contract)
    this.app.post('/api/livekit/token', (req: Request, res: Response) => {
      const { room_id, scene_id, event_id } = req.body;
      
      if (!room_id) {
        res.status(400).json({
          error: 'room_id is required',
        });
        return;
      }
      
      // Validate room_id format (alphanumeric, hyphens, underscores, colons, max 128 chars)
      // Matches production: ^[a-zA-Z0-9_:-]{1,128}$
      if (!/^[a-zA-Z0-9_:-]{1,128}$/.test(room_id)) {
        res.status(400).json({
          error: 'Invalid room_id format',
        });
        return;
      }
      
      // Derive identity from scene/event IDs (deterministic for E2E tests)
      const identityParts: string[] = ['e2e'];
      if (scene_id) {
        identityParts.push(`scene-${scene_id}`);
      }
      if (event_id) {
        identityParts.push(`event-${event_id}`);
      }
      const identity = identityParts.join(':');
      
      const token = this.mockLiveKit.generateToken(room_id, identity);
      const expiresAt = new Date(Date.now() + 300000).toISOString(); // 5 minutes
      
      // Match production response structure (snake_case)
      res.json({
        token,
        expires_at: expiresAt,
      });
    });

    // Simulate latency for testing
    this.app.post('/api/test/simulate-latency', (req: Request, res: Response) => {
      const { roomId, delayMs } = req.body;
      
      if (roomId) {
        this.mockLiveKit.simulateNetworkDelay(roomId, delayMs || 1000);
        res.json({ success: true, roomId, delayMs });
      } else {
        res.status(400).json({ error: 'roomId required' });
      }
    });

    // Simulate packet loss for testing
    this.app.post('/api/test/simulate-packet-loss', (req: Request, res: Response) => {
      const { roomId, lossPercentage } = req.body;
      
      if (roomId) {
        this.mockLiveKit.simulatePacketLoss(roomId, lossPercentage || 10);
        res.json({ success: true, roomId, lossPercentage });
      } else {
        res.status(400).json({ error: 'roomId required' });
      }
    });

    // Get room state for testing
    this.app.get('/api/test/room/:roomId', (req: Request, res: Response) => {
      const roomId = Array.isArray(req.params.roomId) ? req.params.roomId[0] : req.params.roomId;
      const room = this.mockLiveKit.getRoom(roomId);
      
      if (room) {
        res.json({
          id: room.id,
          isLocked: room.isLocked,
          participantCount: room.participants.size,
          participants: Array.from(room.participants.values()).map(p => ({
            identity: p.identity,
            name: p.name,
            isOrganizer: p.isOrganizer,
            isMuted: p.isMuted,
            connectionQuality: p.connectionQuality,
            reconnectCount: p.reconnectCount,
          })),
        });
      } else {
        res.status(404).json({ error: 'Room not found' });
      }
    });
  }

  /**
   * Start both the API server and mock LiveKit server
   */
  async start(): Promise<void> {
    // Start LiveKit mock first
    await this.mockLiveKit.start();
    
    // Then start API server
    return new Promise((resolve) => {
      this.server = this.app.listen(this.port, () => {
        console.log(`[MockAPI] Server started on port ${this.port}`);
        resolve();
      });
    });
  }

  /**
   * Stop both servers
   */
  async stop(): Promise<void> {
    // Stop API server
    if (this.server) {
      await new Promise<void>((resolve) => {
        this.server!.close(() => {
          console.log('[MockAPI] Server stopped');
          resolve();
        });
      });
    }
    
    // Stop LiveKit mock
    await this.mockLiveKit.stop();
  }

  /**
   * Get the Express app for testing
   */
  getApp(): Express {
    return this.app;
  }
}
