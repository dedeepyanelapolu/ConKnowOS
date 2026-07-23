import React, { useState, useEffect, useMemo } from 'react';
import { ReactFlow, Background, Controls } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { Activity, ShieldAlert, Cpu, Server, Coins, Database, Layers } from 'lucide-react';

const INITIAL_NODES = [
  { id: 'Idle', data: { label: 'Idle' }, position: { x: 50, y: 150 }, type: 'input' },
  { id: 'Planning', data: { label: 'Planning' }, position: { x: 220, y: 150 } },
  { id: 'Executing', data: { label: 'Executing' }, position: { x: 400, y: 150 } },
  { id: 'Review', data: { label: 'Review' }, position: { x: 580, y: 150 } },
  { id: 'Completed', data: { label: 'Completed' }, position: { x: 760, y: 50 }, type: 'output' },
  { id: 'Failed', data: { label: 'Failed' }, position: { x: 760, y: 250 }, type: 'output' },
];

const INITIAL_EDGES = [
  { id: 'e-idle-planning', source: 'Idle', target: 'Planning', animated: true },
  { id: 'e-planning-executing', source: 'Planning', target: 'Executing', animated: true },
  { id: 'e-executing-review', source: 'Executing', target: 'Review', animated: true },
  { id: 'e-review-executing', source: 'Review', target: 'Executing', animated: true },
  { id: 'e-review-completed', source: 'Review', target: 'Completed', animated: true },
  { id: 'e-executing-failed', source: 'Executing', target: 'Failed' },
  { id: 'e-review-failed', source: 'Review', target: 'Failed' },
];

export default function Dashboard() {
  const [currentState, setCurrentState] = useState('Idle');
  const [sessionID, setSessionID] = useState('wf_global');
  const [isConnected, setIsConnected] = useState(false);
  const [logs, setLogs] = useState([]);
  const [vectorFacts, setVectorFacts] = useState([]);
  const [tokenMetrics, setTokenMetrics] = useState({
    inputTokens: 0,
    compactTokens: 0,
    costSaved: 0.0,
  });

  useEffect(() => {
    // Establish WebSocket Connection
    const ws = new WebSocket('ws://localhost:8080/ws/trace');

    ws.onopen = () => {
      setIsConnected(true);
      addLog('System', 'Connected to ContextOS WebSocket Broker');
    };

    ws.onclose = () => {
      setIsConnected(false);
      addLog('System', 'Connection closed');
    };

    ws.onerror = (err) => {
      addLog('Error', 'WebSocket connection failed');
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        const { event_type, payload } = data;

        if (event_type === 'EVENT_STATE_CHANGED') {
          setCurrentState(payload.to_state);
          setSessionID(payload.session_id);
          addLog('Runner', `Transitioned from ${payload.from_state} to ${payload.to_state}`);
        } else if (event_type === 'EVENT_TOKEN_COMPACTED') {
          setTokenMetrics({
            inputTokens: payload.input_tokens,
            compactTokens: payload.compact_tokens,
            costSaved: payload.cost_saved,
          });
          addLog('Compactor', `Compacted prompt tokens: ${payload.input_tokens} -> ${payload.compact_tokens}`);
        } else if (event_type === 'EVENT_MEMORY_RETRIEVED') {
          setVectorFacts(payload.vector_facts || []);
          addLog('Memory', `Retrieved ${payload.vector_facts?.length || 0} facts from episodic store`);
        }
      } catch (err) {
        console.error('Failed to parse WebSocket message:', err);
      }
    };

    return () => ws.close();
  }, []);

  const addLog = (module, message) => {
    const timestamp = new Date().toLocaleTimeString();
    setLogs((prev) => [{ timestamp, module, message }, ...prev.slice(0, 49)]);
  };

  // Map state to React Flow nodes with active highlight classes
  const nodes = useMemo(() => {
    return INITIAL_NODES.map((node) => {
      const isActive = node.id === currentState;
      const isCompleted = currentState === 'Completed' && node.id === 'Completed';
      const isFailed = currentState === 'Failed' && node.id === 'Failed';

      let bgStyle = 'bg-cardBg border-white/10 text-gray-400';
      if (isActive) {
        bgStyle = 'bg-blue-600 border-blue-400 text-white active';
      } else if (isCompleted) {
        bgStyle = 'bg-emerald-600 border-emerald-400 text-white';
      } else if (isFailed) {
        bgStyle = 'bg-red-600 border-red-400 text-white';
      }

      return {
        ...node,
        className: `${bgStyle} text-center font-semibold text-sm transition-all duration-300`,
      };
    });
  }, [currentState]);

  // Animate edges pointing to the active node
  const edges = useMemo(() => {
    return INITIAL_EDGES.map((edge) => {
      const isSourceActive = edge.source === currentState;
      const isTargetActive = edge.target === currentState;
      const animated = isSourceActive || isTargetActive;

      return {
        ...edge,
        animated,
        className: animated ? 'active' : '',
      };
    });
  }, [currentState]);

  // Compute stats savings percentage
  const savingsPct = useMemo(() => {
    if (tokenMetrics.inputTokens === 0) return 0;
    const diff = tokenMetrics.inputTokens - tokenMetrics.compactTokens;
    return Math.round((diff / tokenMetrics.inputTokens) * 100);
  }, [tokenMetrics]);

  return (
    <div className="min-h-screen bg-[#0B0F19] text-gray-100 flex flex-col font-sans p-6">
      {/* Header Panel */}
      <header className="flex justify-between items-center pb-6 border-b border-white/5 mb-6">
        <div className="flex items-center gap-3">
          <Activity className="text-blue-500 animate-pulse w-8 h-8" />
          <div>
            <h1 className="text-xl font-bold tracking-tight bg-gradient-to-r from-blue-400 to-indigo-400 bg-clip-text text-transparent">
              ContextOS Observability Dashboard
            </h1>
            <p className="text-xs text-gray-400 mt-0.5">Real-time state and dual-layer memory visualizer</p>
          </div>
        </div>

        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2 px-3 h-9 rounded-lg bg-white/5 border border-white/10 text-xs">
            <span className="text-gray-400">Session:</span>
            <span className="font-mono text-blue-400 font-bold">{sessionID}</span>
          </div>
          <div className={`flex items-center gap-1.5 px-3 h-9 rounded-lg text-xs font-semibold ${
            isConnected ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' : 'bg-red-500/10 text-red-400 border border-red-500/20'
          }`}>
            <span className={`w-2 h-2 rounded-full ${isConnected ? 'bg-emerald-400 animate-ping' : 'bg-red-400'}`}></span>
            {isConnected ? 'ONLINE' : 'OFFLINE'}
          </div>
        </div>
      </header>

      {/* Main Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 flex-1 min-h-[500px]">
        {/* Left Side: Node Graph State Visualizer */}
        <div className="lg:col-span-2 flex flex-col rounded-xl glass-panel overflow-hidden relative">
          <div className="px-4 py-3 border-b border-white/5 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Layers className="w-5 h-5 text-blue-400" />
              <h2 className="text-sm font-semibold">Live State Flow</h2>
            </div>
            <span className="text-xs px-2.5 py-0.5 rounded-full bg-blue-500/10 text-blue-400 font-bold">
              Active: {currentState}
            </span>
          </div>

          <div className="flex-1 w-full min-h-[400px]">
            <ReactFlow nodes={nodes} edges={edges} fitView>
              <Background color="#fff" opacity={0.03} gap={16} />
              <Controls />
            </ReactFlow>
          </div>
        </div>

        {/* Right Side: Metrics, Inspector, and Logs */}
        <div className="flex flex-col gap-6">
          {/* Token Compaction Metrics Panel */}
          <div className="rounded-xl glass-panel p-4 flex flex-col gap-4">
            <div className="flex items-center gap-2">
              <Coins className="w-5 h-5 text-blue-400" />
              <h2 className="text-sm font-semibold">Pruning & Cost Analytics</h2>
            </div>

            <div className="grid grid-cols-3 gap-3">
              <div className="bg-white/5 border border-white/5 rounded-lg p-3 text-center">
                <span className="text-[10px] text-gray-400 block mb-1">Savings</span>
                <span className="text-xl font-extrabold text-blue-400">{savingsPct}%</span>
              </div>
              <div className="bg-white/5 border border-white/5 rounded-lg p-3 text-center">
                <span className="text-[10px] text-gray-400 block mb-1">Compacted</span>
                <span className="text-xl font-extrabold text-indigo-400">{tokenMetrics.compactTokens}t</span>
              </div>
              <div className="bg-white/5 border border-white/5 rounded-lg p-3 text-center">
                <span className="text-[10px] text-gray-400 block mb-1">Cost Saved</span>
                <span className="text-xl font-extrabold text-emerald-400">${tokenMetrics.costSaved.toFixed(5)}</span>
              </div>
            </div>
          </div>

          {/* Memory Hits Inspector */}
          <div className="rounded-xl glass-panel flex-1 flex flex-col overflow-hidden max-h-[300px]">
            <div className="px-4 py-3 border-b border-white/5 flex items-center gap-2">
              <Database className="w-5 h-5 text-blue-400" />
              <h2 className="text-sm font-semibold">Memory Hit Inspector</h2>
            </div>

            <div className="flex-1 overflow-y-auto p-4 flex flex-col gap-3">
              {vectorFacts.length === 0 ? (
                <div className="text-center text-xs text-gray-500 py-12">No active vector memories index fetched</div>
              ) : (
                vectorFacts.map((fact, i) => (
                  <div key={i} className="bg-white/5 border border-white/5 rounded-lg p-3 flex gap-2">
                    <span className="text-[10px] font-bold text-blue-400 bg-blue-500/10 px-1.5 py-0.5 rounded h-fit">FACT</span>
                    <p className="text-xs text-gray-300 leading-relaxed font-mono">{fact}</p>
                  </div>
                ))
              )}
            </div>
          </div>

          {/* Terminal Logs Logger */}
          <div className="rounded-xl glass-panel h-[220px] flex flex-col overflow-hidden">
            <div className="px-4 py-3 border-b border-white/5 flex items-center gap-2">
              <Server className="w-5 h-5 text-blue-400" />
              <h2 className="text-sm font-semibold">Real-Time Event Stream</h2>
            </div>

            <div className="flex-1 overflow-y-auto p-4 font-mono text-[11px] flex flex-col gap-2 bg-[#080B13]">
              {logs.length === 0 ? (
                <div className="text-center text-gray-600 py-12">Waiting for events...</div>
              ) : (
                logs.map((log, i) => (
                  <div key={i} className="flex gap-2 leading-relaxed">
                    <span className="text-gray-500">[{log.timestamp}]</span>
                    <span className="text-blue-400 font-bold">[{log.module}]</span>
                    <span className="text-gray-300">{log.message}</span>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
