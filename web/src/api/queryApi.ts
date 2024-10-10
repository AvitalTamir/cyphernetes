// This is a mock API for now. In a real application, this would make HTTP requests to your backend.

export interface QueryResponse {
  result: string;
  graph: string;
  error?: string;
}

export async function executeQuery(query: string): Promise<QueryResponse> {
  const response = await fetch('/api/query', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ query }),
  });

  if (!response.ok) {
    const data = await response.json();
    throw new Error(data.error);
  }

  const data = await response.json();
  console.log("API response:", data); // Add this line for debugging

  return data;
}

// Add this new function to convert resource names
export async function convertResourceName(name: string): Promise<string> {
  const response = await fetch(`/api/convert-resource-name?name=${encodeURIComponent(name)}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error('Failed to convert resource name');
  }

  const data = await response.json();
  return data.singular;
}

// Update the fetchAutocompleteSuggestions function
export async function fetchAutocompleteSuggestions(query: string, position: number): Promise<string[]> {
  console.log('Sending autocomplete request:', { query, position });
  
  // Convert resource names in the query
  const convertedQuery = await convertQueryResourceNames(query);
  
  const response = await fetch(`/api/autocomplete?query=${encodeURIComponent(convertedQuery)}&position=${position}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error('Failed to fetch autocomplete suggestions');
  }

  const data = await response.json();
  console.log('Received autocomplete response:', data);
  return data.suggestions;
}

// Helper function to convert resource names in the query
async function convertQueryResourceNames(query: string): Promise<string> {
  const regex = /\((\w+):(\w+)\)/g;
  const matches = query.match(regex);
  
  if (!matches) return query;

  for (const match of matches) {
    const [, , resourceName] = match.match(/\((\w+):(\w+)\)/) || [];
    if (resourceName) {
      const singularName = await convertResourceName(resourceName);
      query = query.replace(match, match.replace(resourceName, singularName));
    }
  }

  return query;
}