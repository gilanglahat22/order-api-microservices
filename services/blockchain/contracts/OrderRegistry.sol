// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract OrderRegistry {
    address public owner;
    
    // Order status enum matching our protobuf definitions
    enum OrderStatus {
        UNSPECIFIED,
        CREATED,
        PAYMENT_PENDING,
        PAYMENT_COMPLETED,
        PROVIDER_ASSIGNED,
        PROVIDER_ACCEPTED,
        PROVIDER_REJECTED,
        IN_PROGRESS,
        PICKED_UP,
        IN_TRANSIT,
        ARRIVED,
        DELIVERED,
        COMPLETED,
        CANCELLED,
        REFUNDED,
        DISPUTED
    }
    
    // Order record structure
    struct OrderRecord {
        bytes32 dataHash;
        uint256 timestamp;
        OrderStatus status;
        address updatedBy;
        bool exists;
    }
    
    // Maps order IDs to their latest record
    mapping(string => OrderRecord) public orders;
    
    // Maps order IDs to their history of updates
    mapping(string => OrderRecord[]) public orderHistory;
    
    // Events
    event OrderRecorded(string indexed orderId, bytes32 dataHash, uint256 timestamp, OrderStatus status);
    event OrderUpdated(string indexed orderId, bytes32 dataHash, uint256 timestamp, OrderStatus status);
    
    // Modifiers
    modifier onlyOwner() {
        require(msg.sender == owner, "Only the contract owner can call this function");
        _;
    }
    
    constructor() {
        owner = msg.sender;
    }
    
    // Record a new order
    function recordOrder(string memory orderId, bytes32 dataHash, OrderStatus status) public {
        require(!orders[orderId].exists, "Order already exists");
        
        OrderRecord memory newRecord = OrderRecord({
            dataHash: dataHash,
            timestamp: block.timestamp,
            status: status,
            updatedBy: msg.sender,
            exists: true
        });
        
        orders[orderId] = newRecord;
        orderHistory[orderId].push(newRecord);
        
        emit OrderRecorded(orderId, dataHash, block.timestamp, status);
    }
    
    // Update an existing order
    function updateOrderStatus(string memory orderId, bytes32 dataHash, OrderStatus status) public {
        require(orders[orderId].exists, "Order does not exist");
        
        OrderRecord memory newRecord = OrderRecord({
            dataHash: dataHash,
            timestamp: block.timestamp,
            status: status,
            updatedBy: msg.sender,
            exists: true
        });
        
        orders[orderId] = newRecord;
        orderHistory[orderId].push(newRecord);
        
        emit OrderUpdated(orderId, dataHash, block.timestamp, status);
    }
    
    // Get the current status of an order
    function getOrderStatus(string memory orderId) public view returns (bool exists, bytes32 dataHash, uint256 timestamp, OrderStatus status) {
        OrderRecord memory record = orders[orderId];
        return (record.exists, record.dataHash, record.timestamp, record.status);
    }
    
    // Get the number of history entries for an order
    function getOrderHistoryCount(string memory orderId) public view returns (uint256) {
        return orderHistory[orderId].length;
    }
    
    // Get a specific history entry for an order
    function getOrderHistoryEntry(string memory orderId, uint256 index) public view returns (bytes32 dataHash, uint256 timestamp, OrderStatus status, address updatedBy) {
        require(index < orderHistory[orderId].length, "Index out of bounds");
        OrderRecord memory record = orderHistory[orderId][index];
        return (record.dataHash, record.timestamp, record.status, record.updatedBy);
    }
    
    // Verify if an order's current data hash matches the provided hash
    function verifyOrderHash(string memory orderId, bytes32 dataHash) public view returns (bool) {
        require(orders[orderId].exists, "Order does not exist");
        return orders[orderId].dataHash == dataHash;
    }
    
    // Administrative function to transfer ownership
    function transferOwnership(address newOwner) public onlyOwner {
        require(newOwner != address(0), "New owner cannot be the zero address");
        owner = newOwner;
    }
} 