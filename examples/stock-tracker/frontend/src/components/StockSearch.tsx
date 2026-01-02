import React, { useState } from 'react';

interface StockSearchProps {
  onSearch: (symbol: string) => void;
  loading: boolean;
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: 'flex',
    gap: '10px',
    marginBottom: '20px',
  },
  input: {
    flex: 1,
    padding: '12px 16px',
    fontSize: '1rem',
    borderRadius: '8px',
    border: '2px solid rgba(255, 255, 255, 0.2)',
    background: 'rgba(255, 255, 255, 0.1)',
    color: '#fff',
    outline: 'none',
    transition: 'border-color 0.2s',
  },
  button: {
    padding: '12px 24px',
    fontSize: '1rem',
    fontWeight: '600',
    borderRadius: '8px',
    border: 'none',
    background: '#00d4ff',
    color: '#000',
    cursor: 'pointer',
    transition: 'background 0.2s',
  },
  buttonDisabled: {
    background: '#666',
    cursor: 'not-allowed',
  },
  suggestions: {
    marginTop: '10px',
    display: 'flex',
    gap: '8px',
    flexWrap: 'wrap',
  },
  suggestionChip: {
    padding: '6px 12px',
    borderRadius: '20px',
    background: 'rgba(255, 255, 255, 0.1)',
    border: '1px solid rgba(255, 255, 255, 0.2)',
    color: '#ccc',
    cursor: 'pointer',
    fontSize: '0.85rem',
    transition: 'all 0.2s',
  },
};

const popularStocks = ['AAPL', 'GOOG', 'MSFT', 'AMZN', 'TSLA', 'NVDA', 'META', 'NFLX'];

export default function StockSearch({ onSearch, loading }: StockSearchProps) {
  const [symbol, setSymbol] = useState('');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (symbol.trim()) {
      onSearch(symbol.trim().toUpperCase());
    }
  };

  const handleSuggestionClick = (ticker: string) => {
    setSymbol(ticker);
    onSearch(ticker);
  };

  return (
    <div>
      <form onSubmit={handleSubmit} style={styles.container}>
        <input
          type="text"
          value={symbol}
          onChange={(e) => setSymbol(e.target.value.toUpperCase())}
          placeholder="Enter stock symbol (e.g., AAPL)"
          style={styles.input}
          disabled={loading}
        />
        <button
          type="submit"
          style={{
            ...styles.button,
            ...(loading ? styles.buttonDisabled : {}),
          }}
          disabled={loading}
        >
          {loading ? 'Loading...' : 'Search'}
        </button>
      </form>
      <div style={styles.suggestions}>
        {popularStocks.map((ticker) => (
          <span
            key={ticker}
            style={styles.suggestionChip}
            onClick={() => handleSuggestionClick(ticker)}
          >
            {ticker}
          </span>
        ))}
      </div>
    </div>
  );
}

