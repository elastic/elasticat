import React from 'react';
import { PortfolioData } from '../App';

interface PortfolioProps {
  portfolio: PortfolioData | null;
  onRefresh: () => void;
}

const styles: Record<string, React.CSSProperties> = {
  container: {},
  empty: {
    color: '#666',
    textAlign: 'center',
    padding: '20px',
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: '15px',
  },
  cash: {
    fontSize: '0.9rem',
    color: '#00d4ff',
  },
  refreshButton: {
    padding: '6px 12px',
    borderRadius: '6px',
    border: '1px solid rgba(255, 255, 255, 0.2)',
    background: 'transparent',
    color: '#888',
    cursor: 'pointer',
    fontSize: '0.8rem',
  },
  holdings: {
    display: 'flex',
    flexDirection: 'column',
    gap: '10px',
  },
  holding: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: '12px',
    background: 'rgba(0, 0, 0, 0.3)',
    borderRadius: '8px',
  },
  holdingSymbol: {
    fontWeight: 'bold',
    color: '#00d4ff',
    fontSize: '1rem',
  },
  holdingDetails: {
    fontSize: '0.85rem',
    color: '#888',
    marginTop: '4px',
  },
  holdingValue: {
    textAlign: 'right',
  },
  shares: {
    fontSize: '0.9rem',
    color: '#fff',
  },
  avgCost: {
    fontSize: '0.8rem',
    color: '#888',
  },
};

export default function Portfolio({ portfolio, onRefresh }: PortfolioProps) {
  if (!portfolio) {
    return <div style={styles.empty}>Loading portfolio...</div>;
  }

  if (!portfolio.holdings || portfolio.holdings.length === 0) {
    return (
      <div style={styles.container}>
        <div style={styles.header}>
          <span style={styles.cash}>Cash: ${portfolio.cash.toFixed(2)}</span>
          <button onClick={onRefresh} style={styles.refreshButton}>↻ Refresh</button>
        </div>
        <div style={styles.empty}>No holdings yet. Search for stocks and buy some!</div>
      </div>
    );
  }

  return (
    <div style={styles.container}>
      <div style={styles.header}>
        <span style={styles.cash}>Cash: ${portfolio.cash.toFixed(2)}</span>
        <button onClick={onRefresh} style={styles.refreshButton}>↻ Refresh</button>
      </div>
      <div style={styles.holdings}>
        {portfolio.holdings.map((holding) => (
          <div key={holding.symbol} style={styles.holding}>
            <div>
              <div style={styles.holdingSymbol}>{holding.symbol}</div>
              <div style={styles.holdingDetails}>
                Added {new Date(holding.added_at).toLocaleDateString()}
              </div>
            </div>
            <div style={styles.holdingValue}>
              <div style={styles.shares}>{holding.shares} shares</div>
              <div style={styles.avgCost}>Avg: ${holding.avg_cost.toFixed(2)}</div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

