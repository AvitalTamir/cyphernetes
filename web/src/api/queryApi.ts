// This is a mock API for now. In a real application, this would make HTTP requests to your backend.

export interface QueryResponse {
  result: string;
  graph: any; // You might want to define a more specific type for your graph data
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
    throw new Error('Failed to execute query');
  }

  return response.json();
}