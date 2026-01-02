import React, { useState } from 'react';
import { Stock } from '../App';

interface StockCardProps {
  stock: Stock;
  onAddToPortfolio: (shares: number) => void;
  onAddToWatchlist: () => void;
}

const styles: Record<string, React.CSSProperties> = {
  card: {
    background: 'rgba(0, 0, 0, 0.3)',
    borderRadius: '8px',
    padding: '20px',
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    marginBottom: '15px',
  },
  symbol: {
    fontSize: '1.5rem',
    fontWeight: 'bold',
    color: '#00d4ff',
  },
  name: {
    fontSize: '0.9rem',
    color: '#888',
    marginTop: '4px',
  },
  price: {
    fontSize: '2rem',
    fontWeight: 'bold',
    color: '#fff',
  },
  change: {
    fontSize: '1rem',
    fontWeight: '600',
  },
  positive: {
    color: '#00ff88',
  },
  negative: {
    color: '#ff4444',
  },
  details: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: '10px',
    marginTop: '15px',
    paddingTop: '15px',
    borderTop: '1px solid rgba(255, 255, 255, 0.1)',
  },
  detailItem: {
    fontSize: '0.85rem',
  },
  detailLabel: {
    color: '#888',
  },
  detailValue: {
    color: '#fff',
    fontWeight: '500',
  },
  actions: {
    display: 'flex',
    gap: '10px',
    marginTop: '20px',
  },
  addForm: {
    display: 'flex',
    gap: '8px',
    alignItems: 'center',
  },
  sharesInput: {
    width: '60px',
    padding: '8px',
    borderRadius: '6px',
    border: '1px solid rgba(255, 255, 255, 0.2)',
    background: 'rgba(255, 255, 255, 0.1)',
    color: '#fff',
    fontSize: '0.9rem',
  },
  button: {
    padding: '8px 16px',
    borderRadius: '6px',
    border: 'none',
    cursor: 'pointer',
    fontSize: '0.9rem',
    fontWeight: '500',
  },
  buyButton: {
    background: '#00d4ff',
    color: '#000',
  },
  watchButton: {
    background: 'rgba(255, 255, 255, 0.1)',
    color: '#fff',
    border: '1px solid rgba(255, 255, 255, 0.2)',
  },
};

export default function StockCard({ stock, onAddToPortfolio, onAddToWatchlist }: StockCardProps) {
  const [shares, setShares] = useState(1);
  const isPositive = stock.change >= 0;

  const formatVolume = (vol: number) => {
    if (vol >= 1_000_000) return `${(vol / 1_000_000).toFixed(1)}M`;
    if (vol >= 1_000) return `${(vol / 1_000).toFixed(1)}K`;
    return vol.toString();
  };

  return (
    <div style={styles.card}>
      <div style={styles.header}>
        <div>
          <div style={styles.symbol}>{stock.symbol}</div>
          <div style={styles.name}>{stock.name}</div>
        </div>
        <div style={{ textAlign: 'right' }}>
          <div style={styles.price}>${stock.price.toFixed(2)}</div>
          <div style={{ ...styles.change, ...(isPositive ? styles.positive : styles.negative) }}>
            {isPositive ? '+' : ''}{stock.change.toFixed(2)} ({isPositive ? '+' : ''}{stock.change_pct.toFixed(2)}%)
          </div>
        </div>
      </div>

      <div style={styles.details}>
        <div style={styles.detailItem}>
          <span style={styles.detailLabel}>Volume: </span>
          <span style={styles.detailValue}>{formatVolume(stock.volume)}</span>
        </div>
        <div style={styles.detailItem}>
          <span style={styles.detailLabel}>Updated: </span>
          <span style={styles.detailValue}>
            {new Date(stock.timestamp).toLocaleTimeString()}
          </span>
        </div>
      </div>

      <div style={styles.actions}>
        <div style={styles.addForm}>
          <input
            type="number"
            min="1"
            value={shares}
            onChange={(e) => setShares(Math.max(1, parseInt(e.target.value) || 1))}
            style={styles.sharesInput}
          />
          <button
            onClick={() => onAddToPortfolio(shares)}
            style={{ ...styles.button, ...styles.buyButton }}
          >
            Buy
          </button>
        </div>
        <button
          onClick={onAddToWatchlist}
          style={{ ...styles.button, ...styles.watchButton }}
        >
          + Watchlist
        </button>
      </div>
    </div>
  );
}

