import React, { useState, useEffect } from 'react';
import StockSearch from './components/StockSearch';
import StockCard from './components/StockCard';
import Portfolio from './components/Portfolio';
import Watchlist from './components/Watchlist';

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

export interface Stock {
  symbol: string;
  name: string;
  price: number;
  change: number;
  change_pct: number;
  volume: number;
  timestamp: string;
}

export interface Holding {
  symbol: string;
  shares: number;
  avg_cost: number;
  added_at: string;
}

export interface WatchlistItem {
  symbol: string;
  added_at: string;
}

export interface PortfolioData {
  holdings: Holding[];
  cash: number;
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    maxWidth: '1200px',
    margin: '0 auto',
    padding: '20px',
  },
  header: {
    textAlign: 'center',
    marginBottom: '30px',
  },
  title: {
    fontSize: '2.5rem',
    fontWeight: 'bold',
    color: '#00d4ff',
    marginBottom: '10px',
  },
  subtitle: {
    color: '#888',
    fontSize: '1rem',
  },
  grid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))',
    gap: '20px',
    marginTop: '20px',
  },
  section: {
    background: 'rgba(255, 255, 255, 0.05)',
    borderRadius: '12px',
    padding: '20px',
    border: '1px solid rgba(255, 255, 255, 0.1)',
  },
  sectionTitle: {
    fontSize: '1.2rem',
    fontWeight: '600',
    marginBottom: '15px',
    color: '#fff',
  },
  errorBanner: {
    background: 'rgba(255, 0, 0, 0.2)',
    border: '1px solid rgba(255, 0, 0, 0.5)',
    borderRadius: '8px',
    padding: '15px',
    marginBottom: '20px',
    color: '#ff6b6b',
  },
  chaosSection: {
    marginTop: '30px',
    padding: '20px',
    background: 'rgba(255, 100, 100, 0.1)',
    borderRadius: '12px',
    border: '1px solid rgba(255, 100, 100, 0.3)',
  },
  chaosTitle: {
    fontSize: '1rem',
    fontWeight: '600',
    marginBottom: '10px',
    color: '#ff6b6b',
  },
  chaosButtons: {
    display: 'flex',
    gap: '10px',
  },
  chaosButton: {
    padding: '8px 16px',
    borderRadius: '6px',
    border: 'none',
    cursor: 'pointer',
    fontSize: '0.9rem',
    fontWeight: '500',
  },
};

export default function App() {
  const [selectedStock, setSelectedStock] = useState<Stock | null>(null);
  const [portfolio, setPortfolio] = useState<PortfolioData | null>(null);
  const [watchlist, setWatchlist] = useState<WatchlistItem[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  // Fetch portfolio and watchlist on mount
  useEffect(() => {
    fetchPortfolio();
    fetchWatchlist();
  }, []);

  const fetchPortfolio = async () => {
    try {
      const res = await fetch(`${API_URL}/api/portfolio`);
      if (!res.ok) throw new Error('Failed to fetch portfolio');
      const data = await res.json();
      setPortfolio(data);
    } catch (err) {
      console.error('Portfolio fetch error:', err);
    }
  };

  const fetchWatchlist = async () => {
    try {
      const res = await fetch(`${API_URL}/api/watchlist`);
      if (!res.ok) throw new Error('Failed to fetch watchlist');
      const data = await res.json();
      setWatchlist(data);
    } catch (err) {
      console.error('Watchlist fetch error:', err);
    }
  };

  const handleSearch = async (symbol: string) => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`${API_URL}/api/stocks/${symbol}`);
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || `Failed to fetch ${symbol}`);
      }
      const stock = await res.json();
      setSelectedStock(stock);
    } catch (err: any) {
      setError(err.message);
      setSelectedStock(null);
    } finally {
      setLoading(false);
    }
  };

  const handleAddToPortfolio = async (symbol: string, shares: number, price: number) => {
    try {
      const res = await fetch(`${API_URL}/api/portfolio/${symbol}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ shares, price }),
      });
      if (!res.ok) throw new Error('Failed to add to portfolio');
      await fetchPortfolio();
    } catch (err: any) {
      setError(err.message);
    }
  };

  const handleAddToWatchlist = async (symbol: string) => {
    try {
      const res = await fetch(`${API_URL}/api/watchlist/${symbol}`, {
        method: 'POST',
      });
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to add to watchlist');
      }
      await fetchWatchlist();
    } catch (err: any) {
      setError(err.message);
    }
  };

  const handleRemoveFromWatchlist = async (symbol: string) => {
    try {
      const res = await fetch(`${API_URL}/api/watchlist/${symbol}`, {
        method: 'DELETE',
      });
      if (!res.ok) throw new Error('Failed to remove from watchlist');
      await fetchWatchlist();
    } catch (err: any) {
      setError(err.message);
    }
  };

  // Chaos engineering controls
  const handleChaosKill = async () => {
    try {
      await fetch(`${API_URL}/api/chaos/kill-stock-service`, { method: 'POST' });
      setError('Stock service has been marked unavailable (chaos mode)');
    } catch (err) {
      console.error('Chaos kill failed:', err);
    }
  };

  const handleChaosRestore = async () => {
    try {
      await fetch(`${API_URL}/api/chaos/restore-stock-service`, { method: 'POST' });
      setError(null);
    } catch (err) {
      console.error('Chaos restore failed:', err);
    }
  };

  return (
    <div style={styles.container}>
      <header style={styles.header}>
        <h1 style={styles.title}>📈 Stock Tracker</h1>
        <p style={styles.subtitle}>ElastiCat Demo - Microservices with OpenTelemetry</p>
      </header>

      {error && (
        <div style={styles.errorBanner}>
          <strong>Error:</strong> {error}
          <button
            onClick={() => setError(null)}
            style={{ marginLeft: '10px', cursor: 'pointer', background: 'none', border: 'none', color: '#ff6b6b' }}
          >
            ✕
          </button>
        </div>
      )}

      <StockSearch onSearch={handleSearch} loading={loading} />

      <div style={styles.grid}>
        <div style={styles.section}>
          <h2 style={styles.sectionTitle}>Stock Quote</h2>
          {selectedStock ? (
            <StockCard
              stock={selectedStock}
              onAddToPortfolio={(shares) => handleAddToPortfolio(selectedStock.symbol, shares, selectedStock.price)}
              onAddToWatchlist={() => handleAddToWatchlist(selectedStock.symbol)}
            />
          ) : (
            <p style={{ color: '#666' }}>Search for a stock to see details</p>
          )}
        </div>

        <div style={styles.section}>
          <h2 style={styles.sectionTitle}>Portfolio</h2>
          <Portfolio portfolio={portfolio} onRefresh={fetchPortfolio} />
        </div>

        <div style={styles.section}>
          <h2 style={styles.sectionTitle}>Watchlist</h2>
          <Watchlist
            items={watchlist}
            onRemove={handleRemoveFromWatchlist}
            onSelect={handleSearch}
          />
        </div>
      </div>

      {/* Chaos Engineering Controls */}
      <div style={styles.chaosSection}>
        <h3 style={styles.chaosTitle}>🔥 Chaos Engineering (Demo Controls)</h3>
        <p style={{ color: '#888', fontSize: '0.9rem', marginBottom: '10px' }}>
          Use these to simulate service failures and see errors in ElastiCat logs
        </p>
        <div style={styles.chaosButtons}>
          <button
            onClick={handleChaosKill}
            style={{ ...styles.chaosButton, background: '#ff4444', color: '#fff' }}
          >
            Kill Stock Service
          </button>
          <button
            onClick={handleChaosRestore}
            style={{ ...styles.chaosButton, background: '#44ff44', color: '#000' }}
          >
            Restore Stock Service
          </button>
          <button
            onClick={() => handleSearch('SLOW')}
            style={{ ...styles.chaosButton, background: '#ffaa00', color: '#000' }}
          >
            Test Slow Response
          </button>
          <button
            onClick={() => handleSearch('ERROR')}
            style={{ ...styles.chaosButton, background: '#ff6600', color: '#fff' }}
          >
            Test Error Response
          </button>
          <button
            onClick={() => handleSearch('INVALID')}
            style={{ ...styles.chaosButton, background: '#aa44ff', color: '#fff' }}
          >
            Test 404 Not Found
          </button>
        </div>
      </div>
    </div>
  );
}

