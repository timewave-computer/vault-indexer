export type Json =
  | string
  | number
  | boolean
  | null
  | { [key: string]: Json | undefined }
  | Json[]

export type Database = {
  graphql_public: {
    Tables: {
      [_ in never]: never
    }
    Views: {
      [_ in never]: never
    }
    Functions: {
      graphql: {
        Args: {
          operationName?: string
          query?: string
          variables?: Json
          extensions?: Json
        }
        Returns: Json
      }
    }
    Enums: {
      [_ in never]: never
    }
    CompositeTypes: {
      [_ in never]: never
    }
  }
  public: {
    Tables: {
      block_finality: {
        Row: {
          block_tag: string
          last_validated_block_number: number
        }
        Insert: {
          block_tag: string
          last_validated_block_number: number
        }
        Update: {
          block_tag?: string
          last_validated_block_number?: number
        }
        Relationships: []
      }
      events: {
        Row: {
          block_hash: string
          block_number: number
          contract_address: string
          created_at: string | null
          event_name: string
          id: string
          is_processed: boolean | null
          last_updated_at: string | null
          log_index: number
          raw_data: Json
          transaction_hash: string
        }
        Insert: {
          block_hash: string
          block_number: number
          contract_address: string
          created_at?: string | null
          event_name: string
          id?: string
          is_processed?: boolean | null
          last_updated_at?: string | null
          log_index: number
          raw_data: Json
          transaction_hash: string
        }
        Update: {
          block_hash?: string
          block_number?: number
          contract_address?: string
          created_at?: string | null
          event_name?: string
          id?: string
          is_processed?: boolean | null
          last_updated_at?: string | null
          log_index?: number
          raw_data?: Json
          transaction_hash?: string
        }
        Relationships: []
      }
      positions: {
        Row: {
          amount_shares: string
          contract_address: string
          created_at: string
          id: string
          is_terminated: boolean
          owner_address: string
          position_end_height: number | null
          position_index_id: number
          position_start_height: number
          withdraw_receiver_address: string | null
        }
        Insert: {
          amount_shares: string
          contract_address: string
          created_at?: string
          id?: string
          is_terminated?: boolean
          owner_address: string
          position_end_height?: number | null
          position_index_id: number
          position_start_height: number
          withdraw_receiver_address?: string | null
        }
        Update: {
          amount_shares?: string
          contract_address?: string
          created_at?: string
          id?: string
          is_terminated?: boolean
          owner_address?: string
          position_end_height?: number | null
          position_index_id?: number
          position_start_height?: number
          withdraw_receiver_address?: string | null
        }
        Relationships: []
      }
      rate_updates: {
        Row: {
          block_number: number
          block_timestamp: string
          contract_address: string
          id: string
          rate: string
        }
        Insert: {
          block_number: number
          block_timestamp: string
          contract_address: string
          id?: string
          rate: string
        }
        Update: {
          block_number?: number
          block_timestamp?: string
          contract_address?: string
          id?: string
          rate?: string
        }
        Relationships: []
      }
      withdraw_requests: {
        Row: {
          amount: string
          block_number: number
          contract_address: string
          created_at: string
          id: string
          owner_address: string
          receiver_address: string
          withdraw_id: number
        }
        Insert: {
          amount: string
          block_number: number
          contract_address: string
          created_at?: string
          id?: string
          owner_address: string
          receiver_address: string
          withdraw_id: number
        }
        Update: {
          amount?: string
          block_number?: number
          contract_address?: string
          created_at?: string
          id?: string
          owner_address?: string
          receiver_address?: string
          withdraw_id?: number
        }
        Relationships: []
      }
    }
    Views: {
      [_ in never]: never
    }
    Functions: {
      [_ in never]: never
    }
    Enums: {
      [_ in never]: never
    }
    CompositeTypes: {
      [_ in never]: never
    }
  }
}

type DefaultSchema = Database[Extract<keyof Database, "public">]

export type Tables<
  DefaultSchemaTableNameOrOptions extends
    | keyof (DefaultSchema["Tables"] & DefaultSchema["Views"])
    | { schema: keyof Database },
  TableName extends DefaultSchemaTableNameOrOptions extends {
    schema: keyof Database
  }
    ? keyof (Database[DefaultSchemaTableNameOrOptions["schema"]]["Tables"] &
        Database[DefaultSchemaTableNameOrOptions["schema"]]["Views"])
    : never = never,
> = DefaultSchemaTableNameOrOptions extends { schema: keyof Database }
  ? (Database[DefaultSchemaTableNameOrOptions["schema"]]["Tables"] &
      Database[DefaultSchemaTableNameOrOptions["schema"]]["Views"])[TableName] extends {
      Row: infer R
    }
    ? R
    : never
  : DefaultSchemaTableNameOrOptions extends keyof (DefaultSchema["Tables"] &
        DefaultSchema["Views"])
    ? (DefaultSchema["Tables"] &
        DefaultSchema["Views"])[DefaultSchemaTableNameOrOptions] extends {
        Row: infer R
      }
      ? R
      : never
    : never

export type TablesInsert<
  DefaultSchemaTableNameOrOptions extends
    | keyof DefaultSchema["Tables"]
    | { schema: keyof Database },
  TableName extends DefaultSchemaTableNameOrOptions extends {
    schema: keyof Database
  }
    ? keyof Database[DefaultSchemaTableNameOrOptions["schema"]]["Tables"]
    : never = never,
> = DefaultSchemaTableNameOrOptions extends { schema: keyof Database }
  ? Database[DefaultSchemaTableNameOrOptions["schema"]]["Tables"][TableName] extends {
      Insert: infer I
    }
    ? I
    : never
  : DefaultSchemaTableNameOrOptions extends keyof DefaultSchema["Tables"]
    ? DefaultSchema["Tables"][DefaultSchemaTableNameOrOptions] extends {
        Insert: infer I
      }
      ? I
      : never
    : never

export type TablesUpdate<
  DefaultSchemaTableNameOrOptions extends
    | keyof DefaultSchema["Tables"]
    | { schema: keyof Database },
  TableName extends DefaultSchemaTableNameOrOptions extends {
    schema: keyof Database
  }
    ? keyof Database[DefaultSchemaTableNameOrOptions["schema"]]["Tables"]
    : never = never,
> = DefaultSchemaTableNameOrOptions extends { schema: keyof Database }
  ? Database[DefaultSchemaTableNameOrOptions["schema"]]["Tables"][TableName] extends {
      Update: infer U
    }
    ? U
    : never
  : DefaultSchemaTableNameOrOptions extends keyof DefaultSchema["Tables"]
    ? DefaultSchema["Tables"][DefaultSchemaTableNameOrOptions] extends {
        Update: infer U
      }
      ? U
      : never
    : never

export type Enums<
  DefaultSchemaEnumNameOrOptions extends
    | keyof DefaultSchema["Enums"]
    | { schema: keyof Database },
  EnumName extends DefaultSchemaEnumNameOrOptions extends {
    schema: keyof Database
  }
    ? keyof Database[DefaultSchemaEnumNameOrOptions["schema"]]["Enums"]
    : never = never,
> = DefaultSchemaEnumNameOrOptions extends { schema: keyof Database }
  ? Database[DefaultSchemaEnumNameOrOptions["schema"]]["Enums"][EnumName]
  : DefaultSchemaEnumNameOrOptions extends keyof DefaultSchema["Enums"]
    ? DefaultSchema["Enums"][DefaultSchemaEnumNameOrOptions]
    : never

export type CompositeTypes<
  PublicCompositeTypeNameOrOptions extends
    | keyof DefaultSchema["CompositeTypes"]
    | { schema: keyof Database },
  CompositeTypeName extends PublicCompositeTypeNameOrOptions extends {
    schema: keyof Database
  }
    ? keyof Database[PublicCompositeTypeNameOrOptions["schema"]]["CompositeTypes"]
    : never = never,
> = PublicCompositeTypeNameOrOptions extends { schema: keyof Database }
  ? Database[PublicCompositeTypeNameOrOptions["schema"]]["CompositeTypes"][CompositeTypeName]
  : PublicCompositeTypeNameOrOptions extends keyof DefaultSchema["CompositeTypes"]
    ? DefaultSchema["CompositeTypes"][PublicCompositeTypeNameOrOptions]
    : never

export const Constants = {
  graphql_public: {
    Enums: {},
  },
  public: {
    Enums: {},
  },
} as const

