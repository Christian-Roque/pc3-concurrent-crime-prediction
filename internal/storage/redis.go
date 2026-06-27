package storage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// RedisClient implementa un cliente RESP minimo usando solo libreria estandar.
// Se evita depender de paquetes externos para que el proyecto compile en cualquier equipo.
type RedisClient struct {
	Addr    string
	TTL     time.Duration
	Timeout time.Duration
}

func NewRedisClient(addr string, ttl time.Duration) *RedisClient {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &RedisClient{Addr: addr, TTL: ttl, Timeout: 2 * time.Second}
}

func (c *RedisClient) GetJSON(key string, out any) (bool, error) {
	resp, err := c.Do("GET", key)
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}
	b, ok := resp.([]byte)
	if !ok {
		return false, fmt.Errorf("respuesta Redis inesperada para GET")
	}
	if err := json.Unmarshal(b, out); err != nil {
		return false, err
	}
	return true, nil
}

func (c *RedisClient) SetJSON(key string, value any, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = c.TTL
	}
	seconds := int(ttl.Seconds())
	if seconds <= 0 {
		seconds = 300
	}
	_, err = c.Do("SETEX", key, strconv.Itoa(seconds), string(b))
	return err
}

func (c *RedisClient) Ping() error {
	resp, err := c.Do("PING")
	if err != nil {
		return err
	}
	if s, ok := resp.(string); ok && strings.EqualFold(s, "PONG") {
		return nil
	}
	return fmt.Errorf("respuesta PING inesperada: %v", resp)
}

func (c *RedisClient) Do(args ...string) (any, error) {
	if c == nil || strings.TrimSpace(c.Addr) == "" {
		return nil, fmt.Errorf("redis no configurado")
	}
	conn, err := net.DialTimeout("tcp", c.Addr, c.Timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(c.Timeout))
	if err := writeRESPArray(conn, args); err != nil {
		return nil, err
	}
	return readRESP(bufio.NewReader(conn))
}

func writeRESPArray(w io.Writer, args []string) error {
	if _, err := fmt.Fprintf(w, "*%d\r\n", len(args)); err != nil {
		return err
	}
	for _, arg := range args {
		if _, err := fmt.Fprintf(w, "$%d\r\n%s\r\n", len(arg), arg); err != nil {
			return err
		}
	}
	return nil
}

func readRESP(r *bufio.Reader) (any, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	switch prefix {
	case '+':
		return line, nil
	case '-':
		return nil, fmt.Errorf("redis error: %s", line)
	case ':':
		return strconv.ParseInt(line, 10, 64)
	case '$':
		n, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		if n == -1 {
			return nil, nil
		}
		buf := make([]byte, n+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return buf[:n], nil
	case '*':
		n, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		arr := make([]any, 0, n)
		for i := 0; i < n; i++ {
			v, err := readRESP(r)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("tipo RESP no soportado: %q", prefix)
	}
}
