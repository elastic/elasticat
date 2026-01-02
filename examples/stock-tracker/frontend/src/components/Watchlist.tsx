import React from 'react';
import { WatchlistItem } from '../App';

interface WatchlistProps {
  items: WatchlistItem[];
  onRemove: (symbol: string) => void;
  onSelect: (symbol: string) => void;
}

const styles: Record<string, React.CSSProperties> = {
  container: {},
  empty: {
    color: '#666',
    textAlign: 'center',
    padding: '20px',
  },
  list: {
    display: 'flex',
    flexDirection: 'column',
    gap: '8px',
  },
  item: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: '10px 12px',
    background: 'rgba(0, 0, 0, 0.3)',
    borderRadius: '8px',
    cursor: 'pointer',
    transition: 'background 0.2s',
  },
  symbol: {
    fontWeight: 'bold',
    color: '#00d4ff',
    fontSize: '1rem',
  },
  date: {
    fontSize: '0.8rem',
    color: '#666',
    marginTop: '2px',
  },
  removeButton: {
    padding: '4px 10px',
    borderRadius: '4px',
    border: 'none',
    background: 'rgba(255, 68, 68, 0.2)',
    color: '#ff6b6b',
    cursor: 'pointer',
    fontSize: '0.8rem',
    fontWeight: '500',
  },
};

export default function Watchlist({ items, onRemove, onSelect }: WatchlistProps) {
  if (!items || items.length === 0) {
    return <div style={styles.empty}>No stocks in watchlist. Add some!</div>;
  }

  return (
    <div style={styles.container}>
      <div style={styles.list}>
        {items.map((item) => (
          <div
            key={item.symbol}
            style={styles.item}
            onClick={() => onSelect(item.symbol)}
          >
            <div>
              <div style={styles.symbol}>{item.symbol}</div>
              <div style={styles.date}>
                Added {new Date(item.added_at).toLocaleDateString()}
              </div>
            </div>
            <button
              onClick={(e) => {
                e.stopPropagation();
                onRemove(item.symbol);
              }}
              style={styles.removeButton}
            >
              Remove
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}

